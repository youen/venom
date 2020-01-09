package main

import (
	"github.com/ovh/venom/lib/cmd"
	"github.com/ovh/venom/lib/module"
)

func main() {
	var e Module
	if err := module.Start(e); err != nil { 
		cmd.ExitOnError(err)
	}
}
