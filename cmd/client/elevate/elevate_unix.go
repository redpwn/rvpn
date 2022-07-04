//go:build unix

package elevate

func CheckAdmin() (bool, error) {
	currentUser, err := user.Current()
	return currentUser.Username == "root", err
}
