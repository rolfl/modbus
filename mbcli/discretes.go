package main

type DiscreteGetCommands struct {
	Units   []string `short:"u" long:"unit" description:"Unit(s) to contact" required:"true" env:"MBCLI_UNIT" env-delim:","`
	Timeout int      `short:"t" long:"timeout" default:"5" description:"Timeout (in seconds)"`
	Args    struct {
		Addresses []string `required:"1"`
	} `positional-args:"yes" required:"yes"`
}

func (c *DiscreteGetCommands) Execute(args []string) error {
	return genericClientReads("discrete", c.Units, c.Args.Addresses, c.Timeout)
}

type DiscreteCommands struct {
	Get DiscreteGetCommands `command:"get" alias:"read" description:"Get or read Discrete values"`
}
