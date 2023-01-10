package main

import "fmt"

const helpMsg = `Usage: rvpn <command> [arguments]

Available commands are:

	login      - login and authenticate client
	ls         - list available rVPN profiles
	status     - show current status of rVPN
	connect    - connect to a rVPN profile
	serve      - serve as a rVPN target server
	disconnect - disconnect current rVPN profile
	daemon     - start the rVPN daemon (windows only)
	version    - display rVPN version

Use "rvpn help <command>" for more information about a command
`

func displayHelp() {
	fmt.Print(helpMsg)
}

const connectHelpMsg = `Usage: rvpn connect [profile]

Available flags are:

Advanced flags (only use if you know what you are doing!):
	--subnets - comma separated list of subnets to connect to (will override served subnets)
`

const serveHelpMsg = `Usage: rvpn serve [profile]

Available flags are:
	--subnets - comma separated list of subnets to serve; by default will be all traffic
`

func displayCmdHelp(command string) {
	switch command {
	case "connect":
		fmt.Print(connectHelpMsg)
	case "serve":
		fmt.Print(serveHelpMsg)
	default:
		fmt.Println("Unknown command, run 'rvpn help' for help")
	}
}
