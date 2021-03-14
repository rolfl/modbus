package main

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/rolfl/modbus"
)

var busses = make(map[string]modbus.Modbus)

// 1200, 2400, 4800, 19200, 38400, 57600, and 115200.
var bauds = map[string]int{
	"1200":   1200,
	"2400":   2400,
	"4800":   4800,
	"9600":   9600,
	"19200":  19200,
	"38400":  38400,
	"57600":  57600,
	"115200": 115200,
}

var parities = map[string]int{
	"N": modbus.ParityNone,
	"E": modbus.ParityEven,
	"O": modbus.ParityOdd,
}

var stopbits = map[string]int{
	"1": 1,
	"2": 2,
}

func client(access string) (modbus.Client, error) {
	parts := strings.Split(access, ":")
	if parts[0] == "tcp" {
		if len(parts) != 4 {
			return nil, fmt.Errorf("expect exactly 4 parts for TCP client access tcp:host:port:unit - not: %v", access)
		}
		host := strings.Join(parts[1:3], ":")
		unit, err := strconv.Atoi(parts[3])
		if err != nil {
			return nil, err
		}
		if _, ok := busses[host]; !ok {
			mb, err := modbus.NewTCP(host)
			if err != nil {
				return nil, err
			}
			busses[host] = mb
		}
		return busses[host].GetClient(unit), nil
	}
	if parts[0] == "rtu" {
		if len(parts) < 6 || len(parts) > 7 {
			return nil, fmt.Errorf("expect exactly 4 parts for TCP client access rtu:device:baud:parity:stop:(dtr:)unit - not: %v", access)
		}
		device := parts[1]
		baud, ok := bauds[parts[2]]
		if !ok {
			return nil, fmt.Errorf("illegal baud %v", parts[2])
		}
		parity, ok := parities[parts[3]]
		if !ok {
			return nil, fmt.Errorf("illegal parity %v", parts[3])
		}
		stop, ok := stopbits[parts[4]]
		if !ok {
			return nil, fmt.Errorf("illegal stop bits %v", parts[4])
		}
		dtr := false
		if len(parts) == 7 {
			if parts[5] != "dtr" {
				return nil, fmt.Errorf("DTR must be specified as 'dtr', not %v", parts[5])
			}
			dtr = true
		}
		unit, err := strconv.Atoi(parts[len(parts)-1])
		if err != nil {
			return nil, err
		}
		key := fmt.Sprintf("%v:%v:%v:%v:%v", device, baud, parity, stop, dtr)
		if _, ok = busses[key]; !ok {
			mb, err := modbus.NewRTU(device, baud, parity, stop, dtr)
			if err != nil {
				return nil, err
			}
			busses[key] = mb
		}
		return busses[key].GetClient(unit), nil
	}
	return nil, fmt.Errorf("unknown modbus connection type %v (expect tcp or rtu)", parts[0])
}
