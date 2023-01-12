package main

import (
	"fmt"
	"log"
	"os"

	"github.com/redpwn/rvpn/common"
	"github.com/redpwn/rvpn/daemon"
	"github.com/redpwn/rvpn/service"
	flag "github.com/spf13/pflag"
)

const (
	RVPN_CONTROL_PLANE    = "http://rvpn.jimmyli.us"
	RVPN_CONTROL_PLANE_WS = "ws://rvpn.jimmyli.us"
	RVPN_VERSION          = "0.0.1"
)

func main() {
	// define flags
	subnets := flag.StringSlice("subnets", []string{"0.0.0.0/0"}, "comma separated list of subnets to serve")

	// begin main cli parsing
	flag.Parse()

	err := common.InitRVPNState()
	if err != nil {
		log.Printf("failed to initialize rVPN state: %v", err)
		os.Exit(1)
	}

	if command := flag.Arg(0); command != "" {
		switch command {
		case "help":
			if helpCommand := flag.Arg(1); helpCommand != "" {
				// specific help command was asked
				displayCmdHelp(helpCommand)
			} else {
				// general help
				displayHelp()
			}
		case "login":
			if token := flag.Arg(1); token == "" {
				// no token was provided
				fmt.Println("missing required token, rvpn login [token]")
			} else {
				// token was provided
				ControlPanelAuthLogin(token)
			}
		case "list":
			fmt.Println("list")
		case "connect":
			if profile := flag.Arg(1); profile != "" {
				daemon.EnsureDaemonStarted()
				ClientConnectProfile(profile, common.ClientOptions{
					Subnets: *subnets,
				})
			} else {
				fmt.Println("missing required profile, rvpn connect [profile]")
			}
		case "serve":
			if profile := flag.Arg(1); profile != "" {
				daemon.EnsureDaemonStarted()
				ClientServeProfile(profile)
			} else {
				fmt.Println("missing required profile, rvpn serve [profile]")
			}
		case "disconnect":
			daemon.EnsureDaemonStarted()
			ClientDisconnectProfile()
		case "status":
			daemon.EnsureDaemonStarted()
			ClientStatus()
		case "daemon":
			// start the rVPN daemon which is different based on user operating system
			debug := false
			if debug {
				newDaemon := daemon.NewRVPNDaemon()
				newDaemon.Start()
			} else {
				service.StartRVPNDaemon()
			}
		case "version":
			fmt.Println("rVPN version: " + RVPN_VERSION)
		default:
			fmt.Println("command not found, run 'rvpn help' for help")
		}
	} else {
		fmt.Println("missing required argument, run 'rvpn help' for help")
	}
}
