package modbus

import (
	"errors"
	"fmt"
	"time"
)

type client struct {
	unit  byte
	trans *modbus
	rx    chan pdu
}

// Client is able to drive a single modbus server (Send functions and get responses)
type Client interface {
	// UnitID retrieves the remote unitID we are communicating with
	UnitID() int

	// ReadDiscretes reads read-only discrete values from the remote unit
	ReadDiscretes(from int, count int, tout time.Duration) (*X02xReadDiscretes, error)

	// ReadDiscretes reads coil values from the remote unit
	ReadCoils(from int, count int, tout time.Duration) (*X01xReadCoils, error)
	// WriteSingleCoil writes a single coil values to the remote unit
	WriteSingleCoil(address int, value bool, tout time.Duration) (*X05xWriteSingleCoil, error)
	// WriteMultipleCoils writes multiple coil values to the remote unit
	WriteMultipleCoils(address int, values []bool, tout time.Duration) (*X0FxWriteMultipleCoils, error)

	// ReadInputs reads multiple input values from the remote unit
	ReadInputs(from int, count int, tout time.Duration) (*X04xReadInputs, error)

	// ReadHoldings reads multipls holding register values from a remote unit
	ReadHoldings(from int, count int, tout time.Duration) (*X03xReadHolding, error)
	// WriteSingleHolding writes a single holding register to the remote unit
	WriteSingleHolding(from int, value int, tout time.Duration) (*X06xWriteSingleHolding, error)
	// WriteMultipleHoldings writes multiple holding registers to the remote unit
	WriteMultipleHoldings(address int, values []int, tout time.Duration) (*X10xWriteMultipleHoldings, error)
	// WriteReadMultipleHoldings initially writes one set of holding registers to the remote unit, then in the same
	// operation reads multiple values from the remote unit. The addresses being written and then read do not need to overlap
	WriteReadMultipleHoldings(read int, count int, write int, values []int, tout time.Duration) (*X17xWriteReadHoldings, error)
	// MaskWriteHolding applies an AND mask and an OR mask to a register on the remote unit. The logic is:
	// Result = (Current Contents AND And_Mask) OR (Or_Mask AND (NOT And_Mask))
	MaskWriteHolding(address int, andmask int, ormask int, tout time.Duration) (*X16xMaskWriteHolding, error)
	// Reads a variable number of values from the remote unit's holding register. At most 31 values can be retrieved
	// and the count of values depends on the value at the specified address (if the value at address is 3, it will return the three
	// values that are in address+1, address+2, address+3)
	ReadFIFOQueue(from int, tout time.Duration) (*X18xReadFIFOQueue, error)

	// ReadMultiFileRecords retrieves multiple sequences of File records from the remote unit
	ReadMultiFileRecords(requests []X14xReadRecordRequest, tout time.Duration) (*X14xReadMultiFileRecord, error)
	// ReadFileRecords retrieves a sequence of records from a file on a remote unit
	ReadFileRecords(file int, record int, length int, tout time.Duration) (*X14xReadFileRecordResult, error)
	// WriteMultiFileRecords writes sequences of records to multiple files on a remote unit
	WriteMultiFileRecords(requests []X15xWriteFileRecordRequest, tout time.Duration) (*X15xMultiWriteFileRecord, error)
	// WriteFileRecords writes a sequence of records to a single file on a remote unit
	WriteFileRecords(file int, record int, values []int, tout time.Duration) (*X15xWriteFileRecordResult, error)

	// ReadExceptionStatus returns the exception status register. The value is a bitmask of exception bits, but the meaning
	// of the set bits is device specific (no standard exists).
	ReadExceptionStatus(tout time.Duration) (*X07xReadExceptionStatus, error)
	// ServerID retrieves the ID of the remote unit. This is typically a unique value, but that is not guaranteed.
	ServerID(tout time.Duration) (*X11xServerID, error)
	// DiagnosticRegister retrieves the diagnostic sub-function 2 register. The value is device-specific.
	DiagnosticRegister(tout time.Duration) (*X08xDiagnosticRegister, error)
	// DiagnosticEcho responds with the exact same content that was sent.
	DiagnosticEcho(data []int, tout time.Duration) (*X08xDiagnosticEcho, error)
	// DiagnosticClear resets all counters and logs on the remote unit
	DiagnosticClear(tout time.Duration) error
	// DiagnosticCount retrieves a specific diagnostic counter from the remote unit. See the Diagnostic constants for valid
	// Diagnostic values.
	DiagnosticCount(counter Diagnostic, tout time.Duration) (*X08xDiagnosticCount, error)
	// DiagnosticOverrunClear resets the overrun counter
	DiagnosticOverrunClear(echo int, tout time.Duration) (*X08xDiagnosticOverrunClear, error)
	// CommEventCounter returns the number of "regular" operations on the remote unit. Regular operations access
	// discretes, coils, inputs, registers, and/or files
	CommEventCounter(tout time.Duration) (*X0BxCommEventCounter, error)
	// CommEventLog retrieves the basic details of the most recent 64 messages on the remote unit
	CommEventLog(tout time.Duration) (*X0CxCommEventLog, error)
	// DeviceIdentification retrieves all the remote unit's device labels.
	DeviceIdentification(tout time.Duration) (*X2BxDeviceIdentification, error)
	// DeviceIdentification retrieves a remote unit's specific device label.
	DeviceIdentificationObject(objectID int, tout time.Duration) (*X2BxDeviceIdentificationObject, error)

	// DebugRaw(function byte, payload []byte, tout time.Duration) (*X00xDebugRaw, error)
}

func (c *client) UnitID() int {
	return int(c.unit)
}

type readDecoder func(*dataReader) error

// query is a reuable function that all client-operations uses to coordinate the communication
// with the remote server.
func (c *client) query(tout time.Duration, tx pdu, callback readDecoder) <-chan error {
	errc := make(chan error, 0)
	go func() {
		ticker := time.NewTimer(tout)
		c.trans.txid++
		a := adu{true, c.trans.txid, byte(c.unit), tx}
		select {
		case <-ticker.C:
			errc <- fmt.Errorf("Timeout exceeded waiting to send: %v", tout)
			return
		case c.trans.tx <- a:
			// great, sent the data.....
		}
		select {
		case <-ticker.C:
			errc <- fmt.Errorf("Timeout exceeded waiting to receive: %v", tout)
			return
		case rx := <-c.rx:
			// great, received the data.....
			var err error
			if rx.function >= 128 {
				// error condition
				ec := byte(0)
				if len(rx.data) > 0 {
					ec = rx.data[0]
				}
				switch ec {
				case 1:
					err = errors.New("Modbus Illegal Function")
				case 2:
					err = errors.New("Modbus Illegal Data Address")
				case 3:
					err = errors.New("Modbus Illegal Data Value")
				case 4:
					err = errors.New("Modbus Server Device Failure")
				case 5:
					err = errors.New("Modbus ACK Only")
				case 6:
					err = errors.New("Modbus Server Busy")
				default:
					err = fmt.Errorf("Modbus Unknown error code: %v", ec)
				}
			} else {
				reader := getReader(rx.data)
				err = callback(&reader)
				if err == nil {
					err = reader.remaining()
				}
			}
			errc <- err
			close(errc)
		}
	}()
	return errc
}

func errChan() chan error {
	return make(chan error, 1)
}
