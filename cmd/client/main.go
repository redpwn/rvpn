package main

import (
	"fmt"
	"log"
	"net/rpc"
	"os"

	flag "github.com/spf13/pflag"
)

func main() {
	flag.Parse()

	err := InitRVPNState()
	if err != nil {
		log.Printf("failed to initialize rVPN state: %v", err)
		os.Exit(1)
	}

	if command := flag.Arg(0); command != "" {
		switch command {
		case "help":
			displayHelp()
		case "login":
			if token := flag.Arg(1); token == "" {
				// no token was provided
				fmt.Println("missing required token, rvpn login [token]")
			} else {
				// token was provided
				fmt.Println("token provided")
			}
		case "list":
			fmt.Println("list")
		case "connect":
			if profile := flag.Arg(1); profile != "" {
				EnsureDaemonStarted()

				err := ConnectProfile(profile)
				if err != nil {
					fmt.Println("something went wrong while connecting to " + profile)
					os.Exit(1)
				}
			} else {
				fmt.Println("missing required profile, rvpn connect [profile]")
			}
		case "disconnect":
			EnsureDaemonStarted()

			err := DisconnectProfile()
			if err != nil {
				fmt.Println("something went wrong while disconnecting")
				os.Exit(1)
			}
		case "status":
			EnsureDaemonStarted()

			client, err := rpc.Dial("tcp", "127.0.0.1:52370")
			if err != nil {
				fmt.Println("failed to connect to rVPN daemon")
				os.Exit(1)
			}

			var rVPNState RVPNStatus
			err = client.Call("RVPNDaemon.Status", "", &rVPNState)
			if err != nil {
				fmt.Println("failed to get status from rVPN daemon", err)
				os.Exit(1)
			}

			fmt.Println(rVPNState)
		case "daemon":
			// start the rVPN daemon which is different based on user operating system
			StartRVPNDaemon()
		default:
			fmt.Println("command not found, run 'rvpn help' for help")
		}
	} else {
		fmt.Println("missing required argument, run 'rvpn help' for help")
	}
}
