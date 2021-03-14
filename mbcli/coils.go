package main

import (
	"fmt"
	"time"
)

type CoilGetCommands struct {
	Units   []string `short:"u" long:"unit" description:"Unit(s) to contact" required:"true" env:"MBCLI_UNIT" env-delim:","`
	Timeout int      `short:"t" long:"timeout" default:"5" description:"Timeout (in seconds)"`
	Args    struct {
		Addresses []string `required:"1"`
	} `positional-args:"yes" required:"yes"`
}

func (c *CoilGetCommands) Execute(args []string) error {
	return genericClientReads("coil", c.Units, c.Args.Addresses, c.Timeout)
}

type CoilSetCommands struct {
	Units   []string `short:"u" long:"unit" description:"Unit(s) to contact" required:"true" env:"MBCLI_UNIT" env-delim:","`
	Timeout int      `short:"t" long:"timeout" default:"5" description:"Timeout (in seconds)"`
	Args    struct {
		AddressValues []string `required:"1"`
	} `positional-args:"yes" required:"yes"`
}

func (c *CoilSetCommands) Execute(args []string) error {
	initializeConnections(c.Units)

	timeout := time.Second * time.Duration(c.Timeout)
	addresses, err := addressValues(c.Args.AddressValues, false)
	if err != nil {
		return err
	}

	// run the commands
	for _, sys := range c.Units {
		client, _ := client(sys)
		for _, rng := range addresses {
			flags := make([]bool, len(rng.values))
			for i, v := range rng.values {
				flags[i] = v == 1
			}
			_, err := client.WriteMultipleCoils(rng.address, flags, timeout)
			if err != nil {
				fmt.Printf("Write Holdings: Failed: %v\n", err)
				continue
			}
			got, err := client.ReadCoils(rng.address, len(flags), timeout)
			if err != nil {
				fmt.Printf("Write Holdings verify: Failed: %v\n", err)
			} else {
				fmt.Printf("Write Holdings verify: %v\n", got)
			}
		}
	}
	return nil
}

type CoilCommands struct {
	Get CoilGetCommands `command:"get" alias:"read" description:"Get or read Coil values"`
	Set CoilSetCommands `command:"set" alias:"write" description:"Set or write Coil values"`
}
