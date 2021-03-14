package main

import (
	"fmt"
	"os"

	"github.com/jessevdk/go-flags"
)

type CLICommand struct {
	Verbose    bool               `long:"verbose" description:"Print API requests and responses"`
	Diagnostic DiagnosticCommands `command:"diag" alias:"diagnostics" description:"Diagnostic functions"`
	Discrete   DiscreteCommands   `command:"discrete" alias:"discretes" description:"Discrete functions"`
	Coil       CoilCommands       `command:"coil" alias:"coils" description:"Coil functions"`
	Input      InputCommands      `command:"input" alias:"inputs" description:"Input functions"`
	Holding    HoldingCommands    `command:"holding" alias:"holdings" description:"Holding functions"`
}

func main() {
	clicmd := CLICommand{}

	parser := flags.NewParser(&clicmd, flags.HelpFlag|flags.PassDoubleDash)

	_, err := parser.Parse()

	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
