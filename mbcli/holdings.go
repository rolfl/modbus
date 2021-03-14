package main

import (
	"fmt"
	"time"
)

type HoldingGetCommands struct {
	Units   []string `short:"u" long:"unit" description:"Unit(s) to contact" required:"true" env:"MBCLI_UNIT" env-delim:","`
	Timeout int      `short:"t" long:"timeout" default:"5" description:"Timeout (in seconds)"`
	Args    struct {
		Addresses []string `required:"1"`
	} `positional-args:"yes" required:"yes"`
}

func (c *HoldingGetCommands) Execute(args []string) error {
	return genericClientReads("holding", c.Units, c.Args.Addresses, c.Timeout)
}

type HoldingSetCommands struct {
	Units   []string `short:"u" long:"unit" description:"Unit(s) to contact" required:"true" env:"MBCLI_UNIT" env-delim:","`
	Timeout int      `short:"t" long:"timeout" default:"5" description:"Timeout (in seconds)"`
	Args    struct {
		AddressValues []string `required:"1"`
	} `positional-args:"yes" required:"yes"`
}

func (c *HoldingSetCommands) Execute(args []string) error {
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
			_, err := client.WriteMultipleHoldings(rng.address, rng.values, timeout)
			if err != nil {
				fmt.Printf("Write Holdings: Failed: %v\n", err)
				continue
			}
			got, err := client.ReadHoldings(rng.address, len(rng.values), timeout)
			if err != nil {
				fmt.Printf("Write Holdings verify: Failed: %v\n", err)
			} else {
				fmt.Printf("Write Holdings verify: %v\n", got)
			}
		}
	}
	return nil
}

type HoldingCommands struct {
	Get HoldingGetCommands `command:"get" alias:"read" description:"Get or read Holding values"`
	Set HoldingSetCommands `command:"set" alias:"write" description:"Set or write Holding values"`
}
