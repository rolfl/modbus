package main

type InputGetCommands struct {
	Units   []string `short:"u" long:"unit" description:"Unit(s) to contact" required:"true"`
	Timeout int      `short:"t" long:"timeout" default:"5" description:"Timeout (in seconds)"`
	Args    struct {
		Addresses []string `required:"1"`
	} `positional-args:"yes" required:"yes"`
}

func (c *InputGetCommands) Execute(args []string) error {
	return genericClientReads("input", c.Units, c.Args.Addresses, c.Timeout)
}

type InputCommands struct {
	Get InputGetCommands `command:"get" alias:"read" description:"Get or read Input values"`
}
