package main

import (
	"fmt"
	"os"

	"github.com/redpwn/rvpn/cmd/client/wg"
	flag "github.com/spf13/pflag"
)

func main() {
	flag.Parse()

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
				fmt.Println("TODO NO TOKEN PROVIDED")
			}
		case "list":
			fmt.Println("list")
		case "connect":
			if profile := flag.Arg(1); profile != "" {
				err := wg.ConnectProfile(profile)
				if err != nil {
					fmt.Println("something went wrong while connecting to " + profile)
					os.Exit(1)
				}
			} else {
				fmt.Println("missing required profile, rvpn connect [profile]")
			}
		case "disconnect":
			err := wg.DisconnectProfile()
			if err != nil {
				fmt.Println("something went wrong while disconnecting")
				os.Exit(1)
			}
		case "status":
			rVpnStateLocal, err := wg.GetRVpnState()
			if err != nil {
				fmt.Println("something went wrong while getting state")
				os.Exit(1)
			}

			if rVpnStateLocal.ActiveProfile == "" {
				fmt.Println("not currently connected to a profile")
			} else {
				fmt.Println("currently connected to " + rVpnStateLocal.ActiveProfile)
			}
		default:
			fmt.Println("command not found, run 'rvpn help' for help")
		}
	} else {
		fmt.Println("missing required argument, run 'rvpn help' for help")
	}
}
