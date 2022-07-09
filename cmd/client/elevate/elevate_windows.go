//go:build windows

package elevate

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"syscall"

	"golang.org/x/sys/windows"
)

func CheckAdmin() (bool, error) {
	// taken from: https://coolaj86.com/articles/golang-and-windows-and-admins-oh-my/
	var sid *windows.SID

	err := windows.AllocateAndInitializeSid(
		&windows.SECURITY_NT_AUTHORITY,
		2,
		windows.SECURITY_BUILTIN_DOMAIN_RID,
		windows.DOMAIN_ALIAS_RID_ADMINS,
		0, 0, 0, 0, 0, 0,
		&sid)
	if err != nil {
		log.Fatalf("SID Error: %s", err)
		return false, err
	}

	token := windows.Token(0)

	member, err := token.IsMember(sid)
	if err != nil {
		log.Fatalf("Token Membership Error: %s", err)
		return false, err
	}

	return member, nil
}

func RunWGCmdElevated(cmd string) {
	wgPathRaw, err := exec.Command("where", "wireguard").Output()
	if err != nil {
		fmt.Println("failed to execute cmd", err)
		os.Exit(1)
	}

	wgPath := strings.TrimSuffix(string(wgPathRaw), "\n")

	if wgPath == "INFO: Could not find files for the given pattern(s)." {
		fmt.Println("wireguard not installed, please install and try again")
		os.Exit(1)
	}

	verb := "runas"
	exe := "C:\\Windows\\System32\\cmd.exe"
	cwd, _ := os.Getwd()
	args := `/C "wireguard ` + cmd + `"`

	verbPtr, _ := syscall.UTF16PtrFromString(verb)
	exePtr, _ := syscall.UTF16PtrFromString(exe)
	cwdPtr, _ := syscall.UTF16PtrFromString(cwd)
	argPtr, _ := syscall.UTF16PtrFromString(args)
	showCmd := int32(windows.SW_HIDE)

	err = windows.ShellExecute(0, verbPtr, exePtr, argPtr, cwdPtr, showCmd)
	if err != nil {
		fmt.Println("failed to run wg command", err)
	}
}
