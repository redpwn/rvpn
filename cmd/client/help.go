package main

import "fmt"

const helpMsg = `Usage: rvpn <command> [arguments]

Available commands are:

	login      - login and authenticate client
	ls         - list available rVPN profiles
	status     - show current status of rVPN
	connect    - connect to a rVPN profile
	disconnect - disconnect current rVPN profile
	daemon     - start the rVPN daemon (windows only)

Use "rvpn help <command>" for more information about a command`

func displayHelp() {
	fmt.Println(helpMsg)
}
