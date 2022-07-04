package main

import (
	"fmt"
	"os"

	"github.com/redpwn/rvpn/cmd/client/elevate"
	"github.com/redpwn/rvpn/cmd/client/wg"
	flag "github.com/spf13/pflag"
)

func main() {
	flag.Parse()
	admin, _ := elevate.CheckAdmin()

	err := wg.InitWgClient()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	if command := flag.Arg(0); command != "" {
		switch command {
		case "help":
			displayHelp()
		case "login":
			if token := flag.Arg(1); token == "" {
				// no token was provided
				fmt.Println(token)
			} else {
				// token was provided
			}
		case "list":
			fmt.Println("list")
		case "connect":
			if profile := flag.Arg(1); profile != "" {
				if !admin {
					elevate.RunMeElevated()
				} else {
					// we are in admin mode, attempt to connect
					wg.ConnectProfile(profile)
				}
			} else {
				fmt.Println("missing required profile, rvpn connect [profile]")
			}
		case "disconnect":
			fmt.Println("disconnect")
		default:
			fmt.Println("command not found, run 'rvpn help' for help")
		}
	} else {
		fmt.Println("missing required argument, run 'rvpn help' for help")
	}
}
