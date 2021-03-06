package main

import (
	"fmt"

	"github.com/rolfl/modbus"
)

func updateCoils(server modbus.Server, atomic modbus.Atomic, address int, values []bool, current []bool) ([]bool, error) {
	return values, nil
}

func updateHoldings(server modbus.Server, atomic modbus.Atomic, address int, values []int, current []int) ([]int, error) {
	return values, nil
}

func updateFile(server modbus.Server, atomic modbus.Atomic, file int, address int, values []int, current []int) ([]int, error) {
	return values, nil
}

func main() {
	fmt.Printf("Starting server on all interfaces\n")

	di := []string{"Device Info one", "Device Info two", "Device Info three", "Device Info four"}
	server, err := modbus.NewServer([]byte("Unit test server"), di)
	if err != nil {
		fmt.Printf("Unable to create server: %v\n", err)
		return
	}

	discretes := make([]bool, 5000)
	for id := range discretes {
		discretes[id] = (id & 0x02) != 0
	}

	inputs := make([]int, 500)
	for ii := range inputs {
		inputs[ii] = (ii * 256) & 0xFFFF
	}

	server.RegisterDiscretes(len(discretes))
	server.RegisterCoils(50, updateCoils)
	server.RegisterInputs(len(inputs))
	server.RegisterHoldings(15, updateHoldings)
	server.RegisterFiles(4, updateFile)

	err = server.WriteDiscretesAtomic(0, discretes)
	if err != nil {
		fmt.Printf("Unable to write discretes: %v\n", err)
	}
	err = server.WriteInputsAtomic(0, inputs)
	if err != nil {
		fmt.Printf("Unable to write inputs: %v\n", err)
	}

	tcpserv, err := modbus.NewTCPServer(":502", modbus.ServeAllUnits(server))
	if err != nil {
		fmt.Printf("Unable to create TCP Listener: %v\n", err)
	} else {
		tcpserv.WaitClosed()
	}
}
