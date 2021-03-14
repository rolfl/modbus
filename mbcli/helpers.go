package main

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

type addressedRange struct {
	address int
	count   int
}

func addressRanges(refs []string) ([]addressedRange, error) {
	ret := []addressedRange{}
	for _, ref := range refs {
		parts := strings.Split(ref, ":")
		add, err := strconv.Atoi(parts[0])
		if err != nil {
			return nil, err
		}
		cnt := 1
		if len(parts) > 1 {
			cnt, err = strconv.Atoi(parts[1])
			if err != nil {
				return nil, err
			}
		}
		ret = append(ret, addressedRange{add, cnt})
	}
	return ret, nil
}

type addressedValues struct {
	address int
	values  []int
}

func addressValues(refs []string, isbool bool) ([]addressedValues, error) {
	ret := []addressedValues{}
	for _, ref := range refs {
		parts := strings.Split(ref, ":")
		add, err := strconv.Atoi(parts[0])
		if err != nil {
			return nil, err
		}
		vals := []int{}
		for _, piece := range parts[1:] {
			valstrs := strings.Split(piece, ",")
			for _, sval := range valstrs {
				val, err := strconv.Atoi(sval)
				if err != nil {
					if isbool && (sval == "t" || sval == "true" || sval == "on") {
						val = 1
					} else if isbool && (sval == "f" || sval == "false" || sval == "off") {
						val = 0
					} else {
						return nil, err
					}
				}
				if val < 0 {
					return nil, fmt.Errorf("illegal value %v", sval)
				}
				if isbool && val > 1 {
					return nil, fmt.Errorf("illegal bit value %v", sval)
				}
				vals = append(vals, val)
			}
		}
		ret = append(ret, addressedValues{add, vals})
	}
	return ret, nil
}

func initializeConnections(units []string) error {
	for _, sys := range units {
		_, err := client(sys)
		if err != nil {
			return err
		}
	}
	return nil
}

func genericClientReads(toget string, units []string, addressRefs []string, timeoutSec int) error {
	// initialize the connections
	initializeConnections(units)

	timeout := time.Second * time.Duration(timeoutSec)
	addresses, err := addressRanges(addressRefs)
	if err != nil {
		return err
	}

	// run the commands
	for _, sys := range units {
		client, _ := client(sys)
		var got interface{}
		var name string

		for _, rng := range addresses {
			switch toget {
			case "discrete":
				got, err = client.ReadDiscretes(rng.address, rng.count, timeout)
				name = "Get Discretes"
			case "coil":
				got, err = client.ReadCoils(rng.address, rng.count, timeout)
				name = "Get Coils"
			case "input":
				got, err = client.ReadInputs(rng.address, rng.count, timeout)
				name = "Get Inputs"
			case "holding":
				got, err = client.ReadHoldings(rng.address, rng.count, timeout)
				name = "Get Holding Registers"
			default:
				return fmt.Errorf("unknown read type %v", toget)
			}
			if err != nil {
				fmt.Printf("%v: Failed: %v\n", name, err)
			} else {
				fmt.Printf("%v: %v\n", name, got)
			}
		}
	}
	return nil
}
