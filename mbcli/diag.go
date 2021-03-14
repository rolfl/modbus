package main

import (
	"fmt"
	"time"

	"github.com/rolfl/modbus"
)

type DiagnosticCommands struct {
	ServerID bool     `short:"s" long:"serverid" description:"Return the ServerID value"`
	DeviceID bool     `short:"d" long:"deviceid" description:"Return the DeviceID values"`
	Counts   bool     `short:"c" long:"counts" description:"Return the Diagnostic counter values"`
	Events   bool     `short:"e" long:"events" description:"Return the Event counter value"`
	Clear    bool     `short:"C" long:"clear" description:"Reset the Event counter value"`
	Timeout  int      `short:"t" long:"timeout" default:"5" description:"Timeout (in seconds)"`
	Units    []string `short:"u" long:"unit" description:"Unit(s) to contatc" required:"true"`
}

func (c *DiagnosticCommands) Execute(args []string) error {
	// initialize the connections
	for _, sys := range c.Units {
		_, err := client(sys)
		if err != nil {
			return err
		}
	}

	timeout := time.Second * time.Duration(c.Timeout)

	// run the commands
	for _, sys := range c.Units {
		client, _ := client(sys)
		if c.ServerID {
			if sid, err := client.ServerID(timeout); err != nil {
				fmt.Printf("ServerID: Failed: %v\n", err)
			} else {
				fmt.Printf("ServerID: %v\n", sid)
			}
		}
		if c.DeviceID {
			if did, err := client.DeviceIdentification(timeout); err != nil {
				fmt.Printf("DeviceID: Failed: %v\n", err)
			} else {
				fmt.Printf("DeviceID: %v\n", did)
			}
		}
		if c.Counts {
			counts := []modbus.Diagnostic{
				modbus.BusCommErrors,
				modbus.BusExceptionErrors,
				modbus.BusCharacterOverruns,
				modbus.ServerMessages,
				modbus.ServerNoResponses,
				modbus.ServerNAKs,
				modbus.ServerBusies,
			}
			for _, count := range counts {
				if cnt, err := client.DiagnosticCount(count, timeout); err != nil {
					fmt.Printf("Count %v: Failed: %v\n", count, err)
				} else {
					fmt.Printf("Count: %v\n", cnt)
				}
			}
		}
		if c.Clear {
			if err := client.DiagnosticClear(timeout); err != nil {
				fmt.Printf("Diagnostic Reset: Failed: %v\n", err)
			} else {
				fmt.Printf("Diagnostic counters reset\n")
			}
		}
	}
	return nil
}
