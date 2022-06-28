package main

import (
	"fmt"
	"log"

	"github.com/redpwn/rvpn/cmd/client/wg"
	flag "github.com/spf13/pflag"
)

func main() {
	flag.Parse()

	if command := flag.Arg(0); command != "" {
		switch command {
		case "help":
			displayHelp()
		}
	} else {
		fmt.Println("missing required argument, run 'rvpn help' for help")
	}

	log.Println(wg.Hi())
}
