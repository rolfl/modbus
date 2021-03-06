package main

/*
This program will run the chip through all the API endpoints exposed on the Modbus protocol via the TCP protocol via a protocol gateway

This will run against rhe Phoenix-Contact device gateway directly in it's shared memory, as well as through the gatway to the chip on
the 485 bus behind the gatweway.

The Phoenix contact needs to be configured as a proxy, and also have the shared memory enabled.

Prerequisites:

Chip:
 - SERCOM4
 - PB10 TX
 - PB11 RX
 - 9600-8-E-1

Phoenix Contact
- Lan Settings
  - DHCP
  - Security - defaults
- Serial Settings - Port 1
  - Serial port configuration
	- Port-Mode 485 2-wire - 9600-8-E-1
	- Flow-control - None
	- Terminating Resistor - on
	- DTR off
	- RX timeout between packets - 10ms - the minimum
	- Discard messages with errors - yes
  - Modbus configuration
	- Serial Devices - Modbus RTU Slaves
	- Response timeout - 1000ms
	- Inactivity wait time before TX - 10ms
	- Send write message first - not checked
	- Write mode - Read/Write
	- Device offset mode - off
	- Modbus Exception Response (Irrelevant in this Serial Devices mode) - Disable Gateway
	- Modbus Master/Slaves settings (Irrelevant in this Serial Devices mode) - Unchecked
- Modbus Settings
  - TCP Settings
	- Uncheck everything (default) - only mandatory port 502
  - Remote Modbus Addressing
	- delete everything
  - TCP/IP Connection
	- disable
  - Shared Memory
	- enable
	- Shared Memory Device Id - 247
	- Registers Start address - 400001
	- Coils start address - 1

PC:
 - connected via the USB/Serial debug mode cable to the Xplained board.
 - Windows - Identify the COM port - often COM5 - Search for EDBG in PC's "System Information" application.
 - set to 9600-8-E-1

*/

import (
	"fmt"
	"time"

	"github.com/rolfl/modbus"
)

type processor func() (interface{}, error)

func delay(period time.Duration) {
	tc := time.NewTimer(period)
	<-tc.C
}

func process(reason string, fn processor) {
	fmt.Println("")
	delay(time.Millisecond * 10)
	fmt.Println(reason)
	if val, err := fn(); err != nil {
		fmt.Printf("  Unable to %v: %v\n", reason, err)
	} else {
		fmt.Printf("  %v\n", val)
	}
}

func main() {
	fmt.Printf("Starting Modbus driver\n")
	// mb, err := modbus.NewTCP("modbus:502")
	mb, err := modbus.NewTCP("localhost:502")
	if err != nil {
		fmt.Printf("Error opening modbus: %v\n", err)
		return
	}
	defer mb.Close()

	sm := mb.GetClient(247)

	process("Write Multiple Holding Registers (0004 4)", func() (interface{}, error) {
		vals := []int{111, 222, 333, 444}
		return sm.WriteMultipleHoldings(0, vals, time.Second*2)
	})

	process("Write Multiple Coils (count 5)", func() (interface{}, error) {
		vals := append(make([]bool, 0), false, true, false, true, true, false, false, true, true)
		return sm.WriteMultipleCoils(0, vals, time.Second*2)
	})

	process("Read Coils (-#-##-)", func() (interface{}, error) {
		return sm.ReadCoils(0, 9, time.Second*2)
	})

	process("Read Holdings (FIFO Queue)", func() (interface{}, error) {
		return sm.ReadHoldings(0, 4, time.Second*2)
	})

	c := mb.GetClient(5)

	// for {

	process("Device Identification (strings)", func() (interface{}, error) {
		return c.DeviceIdentification(time.Second * 10)
	})

	process("Diagnostic Count (Server Messages)", func() (interface{}, error) {
		return c.DiagnosticCount(modbus.ServerMessages, time.Second*2)
	})

	// process("Comm Event Log (busy false)", func() (interface{}, error) {
	// return c.CommEventLog(time.Second * 10)
	// })

	process("ServerID (41 42 43 44 45 46 47)", func() (interface{}, error) {
		return c.ServerID(time.Second)
	})

	process("Read Exception Status 00000000", func() (interface{}, error) {
		return c.ReadExceptionStatus(time.Second)
	})

	process("Comm Event Counter (busy false)", func() (interface{}, error) {
		return c.CommEventCounter(time.Second)
	})

	process("Diagnostic Return Query Data ([0001 0002 0003 0004])", func() (interface{}, error) {
		data := []int{1, 2, 3, 4}
		return c.DiagnosticEcho(data, time.Second*2)
	})

	process("Diagnostic Register (0x0000)", func() (interface{}, error) {
		return c.DiagnosticRegister(time.Second * 2)
	})

	process("Diagnostic Clear (OK)", func() (interface{}, error) {
		err := c.DiagnosticClear(time.Second * 2)
		if err != nil {
			return nil, err
		}
		return "OK", nil
	})

	process("Diagnostic Count (Bus Messages)", func() (interface{}, error) {
		return c.DiagnosticCount(modbus.BusMessages, time.Second*2)
	})

	process("Diagnostic Count (Bus Communcation Errors)", func() (interface{}, error) {
		return c.DiagnosticCount(modbus.BusCommErrors, time.Second*2)
	})

	process("Diagnostic Count (Bus Exception Errors)", func() (interface{}, error) {
		return c.DiagnosticCount(modbus.BusExceptionErrors, time.Second*2)
	})

	process("Diagnostic Count (Bus Character Overruns)", func() (interface{}, error) {
		return c.DiagnosticCount(modbus.BusCharacterOverruns, time.Second*2)
	})

	process("Diagnostic Count (Server Messages)", func() (interface{}, error) {
		return c.DiagnosticCount(modbus.ServerMessages, time.Second*2)
	})

	process("Diagnostic Count (Server No Response)", func() (interface{}, error) {
		return c.DiagnosticCount(modbus.ServerNoResponses, time.Second*2)
	})

	process("Diagnostic Count (Server NAK)", func() (interface{}, error) {
		return c.DiagnosticCount(modbus.ServerNAKs, time.Second*2)
	})

	process("Diagnostic Count (Server Busy)", func() (interface{}, error) {
		return c.DiagnosticCount(modbus.ServerBusies, time.Second*2)
	})

	process("Comm Event Log (busy false)", func() (interface{}, error) {
		return c.CommEventLog(time.Second * 2)
	})

	process("Read Discretes (--##--##)", func() (interface{}, error) {
		return c.ReadDiscretes(2000, 8, time.Second*2)
	})

	process("Write Multiple Coils (count 5)", func() (interface{}, error) {
		vals := make([]bool, 0, 5)
		vals = append(vals, false, true, false, true, false)
		return c.WriteMultipleCoils(0, vals, time.Second*2)
	})

	process("Read Coils (-#-#-)", func() (interface{}, error) {
		return c.ReadCoils(0, 5, time.Second*1)
	})

	process("Write Single Coil (0002 -> set/on)", func() (interface{}, error) {
		return c.WriteSingleCoil(2, true, time.Second*1)
	})

	process("Read Coils (-###-)", func() (interface{}, error) {
		return c.ReadCoils(0, 5, time.Second*1)
	})

	process("Read Inputs (0, 256, 512, 768, 1024)", func() (interface{}, error) {
		return c.ReadInputs(0, 5, time.Second*2)
	})

	process("Read FIFO (empty)", func() (interface{}, error) {
		return c.ReadFIFOQueue(5, time.Second*2)
	})

	process("Write Single Holding (FIFO Count)", func() (interface{}, error) {
		return c.WriteSingleHolding(5, 2, time.Second*2)
	})

	process("Write Single Holding (FIFO 1)", func() (interface{}, error) {
		return c.WriteSingleHolding(6, 100, time.Second*2)
	})

	process("Write Single Holding (FIFO 2)", func() (interface{}, error) {
		return c.WriteSingleHolding(7, 200, time.Second*2)
	})

	process("Read Holdings (FIFO Queue)", func() (interface{}, error) {
		return c.ReadHoldings(5, 3, time.Second*2)
	})

	process("Read FIFO (expect 100, 200)", func() (interface{}, error) {
		return c.ReadFIFOQueue(5, time.Second*2)
	})

	process("Read FIFO (expect <empty>)", func() (interface{}, error) {
		return c.ReadFIFOQueue(5, time.Second*2)
	})

	process("Write Multiple Holding Registers (0004 4)", func() (interface{}, error) {
		vals := append(make([]int, 0), 4, 2, 111, 222)
		return c.WriteMultipleHoldings(4, vals, time.Second*2)
	})

	process("Read FIFO (expect 111, 222)", func() (interface{}, error) {
		return c.ReadFIFOQueue(5, time.Second*2)
	})

	process("Read/Write Holding Registers (expect 0x1212, 0, 0, 0, 0, 0, 0, 0, 0, 0)", func() (interface{}, error) {
		data := make([]int, 10)
		data[0] = 0x1212
		return c.WriteReadMultipleHoldings(0, 10, 0, data, time.Second*2)
	})

	process("Mask Write Holding Register (expect 0x0000 0xf2f2 0x2525)", func() (interface{}, error) {
		return c.MaskWriteHolding(0, 0xf2f2, 0x2525, time.Second*2)
	})

	process("Read Holding Register (expect 0x0000 0x1717)", func() (interface{}, error) {
		return c.ReadHoldings(0, 1, time.Second*2)
	})

	process("Read File Record (expect 0x0000 0x0001 )", func() (interface{}, error) {
		return c.ReadFileRecords(0, 0, 5, time.Second*2)
	})

	process("Write File Record (expect 0x0000 0x0001 )", func() (interface{}, error) {
		data := []int{9, 8, 7, 6, 5, 4, 3, 2, 1, 0}
		return c.WriteFileRecords(2, 0, data, time.Second*2)
	})

	process("Read File Record (expect 0x0009 0x0008 0x0007 0x0006 0x0005 0x0004 0x0003 0x0002 0x0001 0x0000 )", func() (interface{}, error) {
		return c.ReadFileRecords(2, 0, 15, time.Second*2)
	})

	process("Write Multi File Record", func() (interface{}, error) {
		reqs := make([]modbus.X15xWriteFileRecordRequest, 3)
		for f := 0; f < 3; f++ {
			r := modbus.X15xWriteFileRecordRequest{}
			r.File = f
			r.Record = 0
			for d := 0; d < 10; d++ {
				r.Values = append(r.Values, (f<<8)|d)
			}
			reqs[f] = r
		}
		return c.WriteMultiFileRecords(reqs, time.Second*2)
	})

	process("Read Multi File Record", func() (interface{}, error) {
		reqs := make([]modbus.X14xReadRecordRequest, 3)
		for f := 0; f < 3; f++ {
			r := modbus.X14xReadRecordRequest{}
			r.File = f
			r.Record = 0
			r.Length = 15
			reqs[f] = r
		}
		return c.ReadMultiFileRecords(reqs, time.Second*2)
	})

	process("Diagnostic Return Query Data (......)", func() (interface{}, error) {
		data := make([]int, 124)
		for i := range data {
			data[i] = i
		}
		return c.DiagnosticEcho(data, time.Second*5)
	})

	process("Device Identification 0x82 (string)", func() (interface{}, error) {
		return c.DeviceIdentificationObject(0x82, time.Second*2)
	})

	process("Device Identification 0x00 (string)", func() (interface{}, error) {
		return c.DeviceIdentificationObject(0x00, time.Second*2)
	})

	process("Device Identification 0x01 (string)", func() (interface{}, error) {
		return c.DeviceIdentificationObject(0x01, time.Second*2)
	})

	process("Device Identification 0x02 (string)", func() (interface{}, error) {
		return c.DeviceIdentificationObject(0x02, time.Second*2)
	})

	process("Device Identification 0x03 (string)", func() (interface{}, error) {
		return c.DeviceIdentificationObject(0x03, time.Second*2)
	})

	process("Device Identification 0x04 (string)", func() (interface{}, error) {
		return c.DeviceIdentificationObject(0x04, time.Second*2)
	})

	process("Device Identification 0x05 (string)", func() (interface{}, error) {
		return c.DeviceIdentificationObject(0x05, time.Second*2)
	})

	process("Device Identification 0x06 (string)", func() (interface{}, error) {
		return c.DeviceIdentificationObject(0x06, time.Second*2)
	})

	process("Device Identification 0x80 (string)", func() (interface{}, error) {
		return c.DeviceIdentificationObject(0x80, time.Second*2)
	})

	process("Device Identification 0x81 (string)", func() (interface{}, error) {
		return c.DeviceIdentificationObject(0x81, time.Second*2)
	})

	process("Device Identification 0x82 (string)", func() (interface{}, error) {
		return c.DeviceIdentificationObject(0x82, time.Second*2)
	})

	delay(2 * time.Second)
	// }
}
