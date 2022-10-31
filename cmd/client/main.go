package main

import (
	"fmt"
	"log"
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
				ClientConnectProfile(profile)
			} else {
				fmt.Println("missing required profile, rvpn connect [profile]")
			}
		case "disconnect":
			EnsureDaemonStarted()
			ClientDisconnectProfile()
		case "status":
			EnsureDaemonStarted()
			ClientStatus()
		case "daemon":
			// start the rVPN daemon which is different based on user operating system
			debug := true
			if debug {
				daemon := NewRVPNDaemon()
				daemon.Start()
			} else {
				StartRVPNDaemon()
			}
		default:
			fmt.Println("command not found, run 'rvpn help' for help")
		}
	} else {
		fmt.Println("missing required argument, run 'rvpn help' for help")
	}
}
