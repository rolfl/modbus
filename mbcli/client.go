package main

import (
	"fmt"
	"strconv"
	"strings"
	"time"

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
		if len(parts) < 6 || len(parts) > 8 {
			return nil, fmt.Errorf("expect 6 to 8 parts for RTU client access rtu:device:baud:parity:stop:(minFrame:)(dtr:)unit - not: %v", access)
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
		idx := 5
		last := len(parts) - 1

		minFrame := 0 * time.Millisecond
		dtr := false
		if idx < last && parts[idx] == "dtr" {
			dtr = true
			idx++
		}
		if idx < last {
			mf, err := strconv.Atoi(parts[idx])
			if err != nil {
				return nil, err
			}
			minFrame = time.Duration(mf) * time.Millisecond
			idx++
		}
		if idx < last {
			return nil, fmt.Errorf("illegal specification - unable to determine last part: %v", access)
		}
		unit, err := strconv.Atoi(parts[idx])
		if err != nil {
			return nil, err
		}
		key := fmt.Sprintf("%v:%v:%v:%v:%v:%v", device, baud, parity, stop, minFrame, dtr)
		if _, ok = busses[key]; !ok {
			mb, err := modbus.NewRTU(device, baud, parity, stop, minFrame, dtr)
			if err != nil {
				return nil, err
			}
			busses[key] = mb
		}
		return busses[key].GetClient(unit), nil
	}
	return nil, fmt.Errorf("unknown modbus connection type %v (expect tcp or rtu)", parts[0])
}
