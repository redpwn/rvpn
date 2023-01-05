package main

import (
	"context"
	"errors"
	"net/netip"
	"time"
)

// syncConnectionPubkey syncs so that the specified rVPN connection is updated in the database
func syncConnectionPubkey(ctx context.Context, db *RVPNDatabase, rVPNConnection RVPNConnection, pubkey string) error {
	rVPNConnection.pubkey = pubkey

	_, err := db.updateConnection(ctx, rVPNConnection.id, rVPNConnection.target, rVPNConnection.deviceId,
		rVPNConnection.pubkey, rVPNConnection.clientIp, rVPNConnection.clientCidr)
	if err != nil {
		return err
	}

	return nil
}

// targetServerAlive returns if the target server a device is connecting to is alive
func targetServerAlive(rVPNTarget *RVPNTarget, connMan *ConnectionManager) bool {
	if rVPNTarget == nil {
		// target does not exist, thus it is not alive
		return false
	}

	if rVPNTarget.serverPubkey == "" || rVPNTarget.serverPublicIp == "" || rVPNTarget.serverPublicVpnPort == "" {
		// target server information does not exist, thus it is not alive
		return false
	}

	// TODO: check server heartbeat

	// check that target is available in the connection manager
	vpnServerConn := connMan.getVPNServerConn(rVPNTarget.name)
	// if nil, vpn server connection was not found
	foundVpnServerConn := vpnServerConn != nil

	return foundVpnServerConn
}

// getNextClientIp returns the next client ip for a target
func getNextClientIp(ctx context.Context, db *RVPNDatabase, target string) (string, string, error) {
	rVPNTarget, err := db.getTargetByName(ctx, target)
	if err != nil {
		return "", "", err
	}

	if rVPNTarget == nil {
		// target does not exist, return new error
		return "", "", errors.New("requested target does not exist")
	}

	clientIpSet, err := db.getTargetClientIps(ctx, target)
	if err != nil {
		return "", "", err
	}

	// we have target information and client ip set, begin calculations for next client ip
	serverIpPrefix, err := netip.ParsePrefix(rVPNTarget.networkIp + rVPNTarget.networkCidr)
	if err != nil {
		return "", "", err
	}

	var ipToAllocate string
	currIp := serverIpPrefix.Addr().Next() // iterate past the first ip because it is reserved for the server

	for serverIpPrefix.Contains(currIp) {
		// iterate while currIp is still contained in the network
		_, exists := clientIpSet[currIp.String()]
		if !exists {
			// we found the ip to allocate
			ipToAllocate = currIp.String()
			break
		}

		currIp = currIp.Next()
	}

	if ipToAllocate == "" {
		// there are no more ips to allocate
		return "", "", errors.New("no available ips to allocate for target")
	}

	// we found an ip to allocate, ensure this is not raced via unique db constraint
	return ipToAllocate, rVPNTarget.networkCidr, nil
}

// blockUntilStale will loop every minute and check if heartbeat is stale (greather than timeout)
func blockUntilStale(ctx context.Context, heartbeatChan chan int, staleTimeout time.Duration) {
	lastHeartbeat := time.Now()
	ticker := time.NewTicker(1 * time.Minute) // check for staleness every minute
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// check if lastHeartbeat is stale
			if time.Since(lastHeartbeat) >= staleTimeout {
				// connection is stale and function should return and unblock
				return
			}
		case <-ctx.Done():
			// context has expired, unblock
			return
		case <-heartbeatChan:
			// we received a heartbeat
			lastHeartbeat = time.Now()
		}
	}
}
