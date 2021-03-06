package modbus

import (
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

type tcpFrame []uint8

type tcp struct {
	name string
	host string
	port int
	conn *net.TCPConn
	// Write to this channel to queue frames to send
	toTX chan adu
	// Frames off the wire will be readable from this channel
	toDemux chan adu
	// whether this is open or not.
	isopen bool
	// a channel that is closed if we are not open ;)
	closed chan bool
	diag   *busDiagnosticManager
}

// NewTCPConn establishes a Modbus transceiver based on a TCP connection
func NewTCPConn(conn *net.TCPConn) (Modbus, error) {
	err := conn.SetKeepAlivePeriod(time.Second * 60)
	if err != nil {
		conn.Close()
		return nil, err
	}
	err = conn.SetKeepAlive(true)
	if err != nil {
		conn.Close()
		return nil, err
	}
	err = conn.SetNoDelay(true)
	if err != nil {
		conn.Close()
		return nil, err
	}

	t := &tcp{}
	t.conn = conn
	t.name = conn.RemoteAddr().String()
	pos := strings.LastIndex(t.name, ":")
	t.port, _ = strconv.Atoi(t.name[pos+1:])
	t.host = t.name[:pos]
	t.isopen = true
	t.closed = make(chan bool, 0)
	t.toDemux = make(chan adu, 0)
	t.toTX = make(chan adu, 0)
	t.diag = newBusDiagnosticManager()

	// start a go routine that reads bytes off the serial device
	go t.wireReader()
	// start a go routine that writes bytes to the serial device
	go t.wireWriter()

	closer := func() error {
		return t.close()
	}

	return newModbus(t.toTX, t.toDemux, closer, t.diag), nil
}

// Close shuts down all communication over the given wires
func (t *tcp) close() error {
	if !t.isopen {
		return nil
	}
	t.isopen = false
	// closing this channel means that anyone readong from the channel is auto-selected in a Select statement
	close(t.closed)
	t.conn.Close()
	return nil
}

// wireRead takes data off the wire, and submits complete frames to the RTU.rx channel.
// It manages the TX idle timer as well, so that we cannot send data until the bus is idle.
func (t *tcp) wireReader() {
	noDeadline := time.Time{}
	buffer := make([]uint8, 300)

	err := t.conn.SetReadDeadline(noDeadline)
	if err != nil {
		fmt.Printf("Shutting down reading: %v\n", err)
		t.close()
		return
	}

	/*
		We expect that 99.9% of the time, a TCP packet will have 1, and only 1 modbus frame in it, and it will
		be a complete frame.
	*/

	got := 0
	expect := 7
	ok := true
	for {
		n := 0
		if got < expect {
			// there may be a delay set on this read if there's more data needed to read a frame.
			n, err = t.conn.Read(buffer[got:])
			if err != nil && !errors.Is(err, os.ErrDeadlineExceeded) {
				fmt.Printf("Shutting down reading: %v\n", err)
				t.close()
				break
			}
			// if there was a deadline, we remove it.
			err := t.conn.SetReadDeadline(noDeadline)
			if err != nil {
				fmt.Printf("Shutting down reading: %v\n", err)
				t.close()
				break
			}
		}
		got += n
		if got >= 7 {
			// we have enough data for some initial checks.
			if ck := getWord(buffer, 2); ck != 0 {
				fmt.Printf("Expect MODBUS protocol 0 top be set. Not 0x%04x\n", ck)
				ok = false
				t.diag.commError()
			}
			if pduszp := getWord(buffer, 4) - 1; pduszp > 253 {
				fmt.Printf("Expect PDU Payload to not exceed 253 bytes. Not 0x%04x\n", pduszp)
				ok = false
				t.diag.overrun()
			} else {
				expect = int(pduszp) + 7
			}
		}
		if ok {
			if got >= expect {
				// we have a full frame of data.... perhaps more.
				frame := make([]uint8, expect)
				copy(frame, buffer)
				// frame is populated, let's send it to the handler.
				if validFrame(t.name, frame) {
					f := decodeTCPFrame(frame)
					t.diag.message(f.unit == 0)
					t.toDemux <- f
				}
				// Copy and data to the beginning of the next frame
				copy(buffer, buffer[expect:got])
				// reset our counters to read the next frame
				got = got - expect
				expect = 7
			} else {
				// we expect more data.......
				// for the remaining data, we have a read timeout.
				t.conn.SetReadDeadline(time.Now().Add(time.Second))
			}
		} else {
			// problem with the frame
			n = 0
			got = 0
			expect = 7
		}
	}
	fmt.Printf("Terminating tcp reader %s: closed\n", t.name)
}

// wireWriter takes data off the wire, and submits complete frames to the RTU.rx channel.
// It manages the TX idle timer as well, so that we cannot send data until the bus is idle.
func (t *tcp) wireWriter() {
	alive := true
	for alive {
		// fmt.Println("Waiting for data to send on TX")
		select {
		case <-t.closed:
			alive = false
		case ta := <-t.toTX:
			// data to send.... let's wait for the channel to be ready....
			// fmt.Println("Got data to send on TX, waiting for TX IDLE")
			if !ta.request {
				t.diag.response(ta.pdu)
			}
			f := buildTCPFrame(ta)
			for len(f) > 0 {
				if n, err := t.conn.Write(f); err != nil {
					// fmt.Printf("Unable to send bytes to %s: %s\n", rtu.name, err)
					f = f[:0]
				} else {
					f = f[n:]
				}
			}
		}
	}
	fmt.Printf("Terminating TCP writer %s: closed\n", t.name)
}

func validFrame(name string, tdata []byte) bool {
	if len(tdata) == 0 {
		return false
	}
	if len(tdata) < 7 {
		fmt.Printf("Too small of a frame on %s, just %d bytes\n", name, len(tdata))
		return false
	}
	if len(tdata) > 260 {
		fmt.Printf("Too large of a frame on %s, %d exceeds 260 bytes\n", name, len(tdata))
		return false
	}
	return true
}

func decodeTCPFrame(tdata []byte) adu {
	// OK, we have a frame, send it to the respective client.
	// to keep things similar to RTU framing, we slice the data at pos 6, the unit address....
	p := pdu{tdata[7], tdata[8:]}
	tid := getWord(tdata, 0)
	a := adu{false, tid, tdata[6], p}
	return a
}

func buildTCPFrame(td adu) []byte {
	pdu := td.pdu
	payload := 1 + len(pdu.data)
	sz := 7 + payload // data plus address, control, and function bytes
	data := make([]uint8, sz)
	setWord(data, 0, td.txid)
	setWord(data, 2, 0) // protocol identifier - always 0 for Modbus
	setWord(data, 4, uint16(1+payload))
	setByte(data, 6, td.unit)
	setByte(data, 7, pdu.function)
	copy(data[8:], pdu.data)
	return data
}
