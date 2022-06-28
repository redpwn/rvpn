package main

import "fmt"

const helpMsg = `Usage: rvpn <command> [arguments]

Available commands are:

	login      - login and authenticate client
	ls         - list available rVPN profiles
	connect    - connect to a rVPN profile
	disconnect - disconnect current rVPN profile

Use "rvpn help <command>" for more information about a command`

func displayHelp() {
	fmt.Println(helpMsg)
}
