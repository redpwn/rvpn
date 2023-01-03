//go:build linux

package elevate

import "os/user"

func CheckAdmin() (bool, error) {
	currentUser, err := user.Current()
	return currentUser.Username == "root", err
}
