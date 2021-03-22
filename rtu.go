package modbus

import (
	"fmt"
	"time"

	"github.com/rolfl/modbus/serial"
)

const (
	// ParityNone is what is used to have no parity bit
	ParityNone = 'N'
	// ParityOdd is what is used to have one parity bit that is set to have an even number of 1 bits
	ParityOdd = 'O'
	// ParityEven is what is used to have one parity bit that is set to have an odd nuber of 1 bits
	ParityEven = 'E'
)

const (
	// StopBitsOne ensures a single stop bit after transmitting data.
	StopBitsOne = 1
	// StopBitsTwo ensures 2 stop bits after transmitting data.
	StopBitsTwo = 2
)

type rtu struct {
	name string
	// internally used to feed each char as it comes off the wire
	rxchar chan byte
	// channel has a value when there's been a pause in reception (end of frame?)
	rxto chan bool
	// reset the reception pause clock... presumably because we got a new character
	rxtoc chan bool
	// will have a value on it if the wire is ready for a transmission
	txready chan bool
	// How long after an RX character to wait for end of frame
	pause time.Duration
	// How long after an End of frame to wait before we can write
	idle time.Duration
	// The serial port we talk over.
	serial *serial.Port
	// whether this is open or not.
	isopen bool
	// a channel that is closed if we are not open ;)
	closed chan bool
	// Things we have received from the modbus, but need to send to the demuxer
	toDemux chan adu
	// Things that need to be sent to the modbus
	toTX chan adu
	// ID to use for uncorrelated calls
	txid uint16
	// wlog chan wirelog
	// check whether incoming packets are associated with outgoing calls.
	pending map[byte]uint16
	diag    *busDiagnosticManager
}

// NewRTU establishes a connection to a local COM port (windows) or serial device (others)
func NewRTU(device string, baud int, parity int, stopbits int, minFrame time.Duration, dtr bool) (Modbus, error) {
	options := serial.Config{}
	options.Name = device
	options.Baud = baud
	options.Size = 8
	options.Parity = serial.ParityNone

	switch parity {
	case 'N':
		options.Parity = serial.ParityNone
	case 'E':
		options.Parity = serial.ParityEven
	case 'O':
		options.Parity = serial.ParityOdd
	default:
		return nil, fmt.Errorf("illegal parity %c", parity)
	}
	switch stopbits {
	case 1:
		options.StopBits = serial.Stop1
	case 2:
		options.StopBits = serial.Stop2
	default:
		return nil, fmt.Errorf("illegal stop bits %v", stopbits)
	}

	options.ReadTimeout = time.Millisecond

	port, err := serial.OpenPort(&options)
	if err != nil {
		return nil, err
	}

	if dtr {
		err = port.SetDTR()
		if err != nil {
			return nil, err
		}
	}

	fmt.Printf("Opened Modbus RTU on %v at %v-%c-%v\n", device, baud, parity, stopbits)
	wp := rtu{}
	wp.name = device
	wp.serial = port
	wp.isopen = true
	wp.closed = make(chan bool)
	wp.rxchar = make(chan byte, 300)
	wp.rxto = make(chan bool)
	wp.rxtoc = make(chan bool)
	wp.txready = make(chan bool, 1)
	wp.toTX = make(chan adu, 5)
	wp.toDemux = make(chan adu, 5)
	wp.pending = make(map[byte]uint16)
	wp.diag = newBusDiagnosticManager()
	// wp.wlog = make(chan wirelog, 10)

	// From the Modbus spec, wait 1.5 chars for frame end, and 3.5 for bus idle
	// For baud rates greater than 19200 Bps, fixed values for the 2 timers should be used: it is
	// recommended to use a value of 750µs for the inter-character time-out (t1.5) and a value of
	// 1.750ms for inter-frame delay (t3.5).
	bc := 8 + stopbits
	if parity != 'N' {
		bc++
	}
	// hc is the time for half a char
	hc := time.Duration((float64(bc) / float64(baud)) * (1000000.0 * float64(time.Microsecond)))
	// 3 halfchars is 1.5 chars
	wp.pause = 3 * hc
	// add another 4 halfchars to get 3.5 chars.
	wp.idle = 4 * hc

	if wp.pause < 1*time.Millisecond {
		wp.pause = 1 * time.Millisecond
	}

	if wp.idle < 2*time.Millisecond {
		wp.idle = 2 * time.Millisecond
	}

	// Set the frame-detect pause to the minimum pause if set.
	if wp.pause < minFrame {
		wp.pause = minFrame
	}

	closer := func() error {
		return wp.close()
	}

	// start a go routine that reads bytes off the serial device
	go wp.wireReader()
	// start a go routine that writes bytes to the serial device
	go wp.wireWriter()
	// start a go routine that manages the clocks.....
	go wp.ticker()
	// start a go routine that frames up received messages.
	go wp.wireFramer()

	// go wp.wireLogger()

	return newModbus(wp.toTX, wp.toDemux, closer, wp.diag), nil
}

func (rtu *rtu) close() error {
	if !rtu.isopen {
		return nil
	}
	rtu.isopen = false
	// closing this channel means that anyone reading from the channel is auto-selected in a Select statement
	close(rtu.closed)
	rtu.serial.Close()
	return nil
}

// wireFramer reads data from the wireReader channel, and waits for the frame token too.
// it processes received frames, validates them, etc. then distributes them to the respective clients.
func (rtu *rtu) wireFramer() {
	alive := true
	for alive {
		data := make([]byte, 0, 300)
		framedone := false
		for !framedone {
			select {
			case ch := <-rtu.rxchar:
				// we cheat a bit, add chars to a certain length, then start bitbucketing them.
				// the actual frame-size check happens in handleFrame
				if len(data) < 260 {
					data = append(data, ch)
				}
				// fmt.Printf("%0x\n", ch)
			case <-rtu.rxto:
				// we have a frame.... check it, and distribute it.
				// fmt.Printf("<<<%v - %v\n", len(data), data)
				rtu.handleFrame(data)
				framedone = true
			}
		}
	}
}

func (rtu *rtu) handleFrame(frame rtuFrame) {
	if len(frame) == 0 {
		return
	}
	if len(frame) < 4 {
		fmt.Printf("Too small of a frame on %s, just %d bytes\n", rtu.name, len(frame))
		rtu.diag.commError()
		return
	}
	if len(frame) > 256 {
		rtu.diag.overrun()
		fmt.Printf("Too large of a frame on %s, exceeds 256 bytes\n", rtu.name)
		return
	}

	xcrc := computeCRC16(frame[:len(frame)-2])
	gcrc := getWordLE(frame, len(frame)-2)
	if xcrc != gcrc {
		fmt.Printf("CRC Mismatch on %s. Expected %d but got %d\n", rtu.name, xcrc, gcrc)
		rtu.diag.commError()
		return
	}

	// OK, we have a frame, send it to the respective client.
	unit := frame[0]
	function := frame[1]
	data := frame[2 : len(frame)-2]

	rtu.diag.message(unit == 0)

	p := pdu{function, data}
	a := adu{false, 0, unit, p}
	if txid, ok := rtu.pending[unit]; ok {
		a.txid = txid
		delete(rtu.pending, unit)
	} else {
		rtu.txid++
		a.txid = rtu.txid
	}

	rtu.toDemux <- a
}

const (
	waitframe = iota
	waitidle
	isidle
)

// ticker monitors the state of the wire, and identifies full frames being received, and when it's safe to transmit.
func (rtu *rtu) ticker() {
	// initial state is S
	mode := waitidle
	// set up a timer - wait at least a second for the bus to be idle, but stop it immediately.
	tc := time.NewTimer(time.Second)
	for {
		tc.Stop()

		switch mode {
		case waitframe:
			// reset the timer to wait for the end of the frame
			tc.Reset(rtu.pause)
		case waitidle:
			// after getting a frame, we wait for an idle period too.
			tc.Reset(rtu.idle)
		case isidle:
			// we can stop the timer - we're not waiting for anything.
		}

		select {
		case <-rtu.closed:
			return
		case <-rtu.rxtoc:
			// rxtoc is pinged when bytes are received.
			mode = waitframe
		case <-tc.C:
			if mode == waitidle {
				// We have a prolonged period where the bus is idle after bus activity
				// we can now write to the bus if we need to (3.5 char period)
				//fmt.Println("Tock")
				rtu.txready <- true
				// set the mode to Off.
				mode = isidle
				// fmt.Printf("Long Idle marker\n")
			}
			if mode == waitframe {
				// We have received a short idle period 1.5 chars, frame is done.
				//fmt.Println("Tick")
				rtu.rxto <- true
				// set the mode to wait for idle.
				mode = waitidle
				// fmt.Printf("Short Idle marker\n")
			}
		}
	}
}

// wireRead takes data off the wire, and submits complete frames to the RTU.rx channel.
// It manages the TX idle timer as well, so that we cannot send data until the bus is idle.
func (rtu *rtu) wireReader() {
	alive := true
	buffer := make([]byte, 256)
	for alive {
		n, err := rtu.serial.Read(buffer)
		if err != nil {
			fmt.Printf("Error reading from serial line %s: %s\n", rtu.name, err)
			n = 0
		}
		if n != 0 {
			// cp := make([]byte, n)
			// copy(cp, buffer)
			// rtu.wlog <- wirelog{time.Now(), cp}
			// reset the clock timeout.
			rtu.rxtoc <- true
			// send the chars to the channel
			for _, ch := range buffer[:n] {
				rtu.rxchar <- ch
			}
			// also, we tell transmitters to wait more.
			select {
			case <-rtu.txready:
				// we were previously advertising that a transmission could happen....
				// do nothing, but know that a tranmission will now have to wait for another idle window to send
				// we have consumed the token, though, so the writer cannot have it... ;)
			default:
				// we were not advertising that it could happen
				// do nothing.
			}
		}
		// Some channel magic, if the rtu.closed channel is closed, it's automatically selected.
		select {
		case <-rtu.closed:
			alive = false
		default:
			// Nothing to see here, move along.
		}
	}
	fmt.Printf("Terminating serial line reader %s: closed\n", rtu.name)
}

// func (rtu *rtu) wireLogger() {
// 	prev := time.Now()
// 	for l := range rtu.wlog {
// 		dur := l.at.Sub(prev)
// 		fmt.Printf("Received at %v (delay %v): %v\n", l.at, dur, l.bytes)
// 		prev = l.at
// 	}
// }

// wireWriter takes frames that are ready to send, waits for an idle period on the wire, and transmits it.
func (rtu *rtu) wireWriter() {
	alive := true
	for alive {
		// fmt.Println("Waiting for data to send on TX")
		select {
		case <-rtu.closed:
			alive = false
		case f := <-rtu.toTX:
			// data to send.... let's wait for the channel to be ready....
			// fmt.Println("Got data to send on TX, waiting for TX IDLE")
			if f.request {
				rtu.pending[f.unit] = f.txid
			}
			select {
			case <-rtu.closed:
				alive = false
			case <-rtu.txready:
				// wire is clear to send on... let's dump it.
				// fmt.Println("Got TX IDLE, waiting for TX COMPLETE")
				if !f.request {
					rtu.diag.response(f.pdu)
				}
				frame := buildRTUFrame(f)
				for len(frame) > 0 {
					if n, err := rtu.serial.Write(frame); err != nil {
						// fmt.Printf("Unable to send bytes to %s: %s\n", rtu.name, err)
						frame = frame[:0]
					} else {
						frame = frame[n:]
					}
				}
			}
		}
	}
	fmt.Printf("Terminating serial line writer %s: closed\n", rtu.name)
}

func buildRTUFrame(f adu) rtuFrame {
	sz := len(f.pdu.data) + 4 // data plus address and function bytes and 2 CRC bytes
	data := make([]byte, sz)
	data[0] = f.unit
	data[1] = f.pdu.function
	copy(data[2:], f.pdu.data)
	crc := computeCRC16(data[:sz-2])
	setWordLE(data, sz-2, crc)
	return data
}
