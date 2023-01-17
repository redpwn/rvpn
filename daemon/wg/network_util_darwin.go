//go:build darwin

package wg

import (
	"log"
	"os/exec"
	"regexp"
	"strings"
)

// assignInterfaceAddr assigns an ip to an interface using ifconfig
func assignInterfaceAddr(ifaceName, ipAddressPrefix, ipAddressNet string) error {
	cmd := exec.Command("ifconfig", ifaceName, "inet", ipAddressPrefix, ipAddressNet)
	if out, err := cmd.CombinedOutput(); err != nil {
		log.Printf("adding address command \"%v\" failed with output %s and error: ", cmd.String(), out)
		return err
	}

	return nil
}

// findDefaultInterface returns the name of the default interface name and gateway ip
func findDefaultInterface() (interfaceName string, gateway string, err error) {
	cmd := exec.Command("netstat", "-nr")
	out, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("failed to get routes from netstat command \"%v\" failed with output %s and error:", cmd.String(), out)
		return "", "", err
	}

	splitOut := strings.Split(string(out), "\n")
	re := regexp.MustCompile(`\s+`)

	for _, routeLine := range splitOut {
		routeLine := strings.TrimSpace(routeLine)
		splitLine := re.Split(routeLine, 6)

		if len(splitLine) == 4 || len(splitLine) == 5 {
			// most likely this is a routing line (this property is not guaranteed)
			dst := splitLine[0]
			gateway := splitLine[1]
			interfaceName := splitLine[3]

			if dst == "default" {
				return interfaceName, gateway, nil
			}
		}
	}

	return "", "", nil
}

// routeAddIFace adds a route using the interface as the next hop
func routeAddIFace(ipAddressPrefix, interfaceName string) error {
	cmd := exec.Command("route", "add", "-net", ipAddressPrefix, "-interface", interfaceName)
	if out, err := cmd.CombinedOutput(); err != nil {
		log.Printf("route add command \"%v\" failed with output %s and error: ", cmd.String(), out)
		return err
	}

	return nil
}

func routeAddGateway(ipAddressPrefix, gatewayAddressPrefix string) error {
	cmd := exec.Command("route", "add", "-net", ipAddressPrefix, gatewayAddressPrefix)
	if out, err := cmd.CombinedOutput(); err != nil {
		log.Printf("route add command \"%v\" failed with output %s and error: ", cmd.String(), out)
		return err
	}

	return nil
}

// routeDelIFace deletes the route for a specified interface
func routeDelIFace(ipAddressPrefix, interfaceName string) error {
	cmd := exec.Command("route", "delete", "-net", ipAddressPrefix, "-interface", interfaceName)
	if out, err := cmd.CombinedOutput(); err != nil {
		log.Printf("route del command \"%v\" failed with output %s and error: ", cmd.String(), out)
		return err
	}

	return nil
}

func routeDelGateway(ipAddressPrefix, gatewayAddressPrefix string) error {
	cmd := exec.Command("route", "delete", "-net", ipAddressPrefix, gatewayAddressPrefix)
	if out, err := cmd.CombinedOutput(); err != nil {
		log.Printf("route add command \"%v\" failed with output %s and error: ", cmd.String(), out)
		return err
	}

	return nil
}
