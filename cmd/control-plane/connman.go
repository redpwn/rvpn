package main

import "github.com/sourcegraph/jsonrpc2"

type ConnectionManager struct {
	vpnServerConnections map[string]*jsonrpc2.Conn   // targetName : jrpc connection
	vpnClientConnections map[string][]*jsonrpc2.Conn // targetName : [] jrpc connection for all targets
	// TODO: determine if vpnClientConnections needs more identifiers
	// TODO: synchronization / locking for connections maps
}

func NewConnectionManager() *ConnectionManager {
	return &ConnectionManager{
		vpnServerConnections: make(map[string]*jsonrpc2.Conn),
		vpnClientConnections: make(map[string][]*jsonrpc2.Conn),
	}
}

// setVPNServerConn sets the given connection as the serving connection for the target
// returns true if old target connection was overwritten, else false
func (c *ConnectionManager) setVPNServerConn(targetName string, conn *jsonrpc2.Conn) bool {
	_, exists := c.vpnServerConnections[targetName]

	c.vpnServerConnections[targetName] = conn

	return exists
}

// setVPNClientConn appends the given connection as a client for the target
func (c *ConnectionManager) setVPNClientConn(targetName string, conn *jsonrpc2.Conn) {
	_, exists := c.vpnClientConnections[targetName]

	if exists {
		// there is already a list of targets for the connection
		c.vpnClientConnections[targetName] = append(c.vpnClientConnections[targetName], conn)
	} else {
		// list of targets does not yet exist for the connection
		c.vpnClientConnections[targetName] = []*jsonrpc2.Conn{conn}
	}
}

// getVPNServerConn gets the server jrpc connection for the target
func (c *ConnectionManager) getVPNServerConn(targetName string) *jsonrpc2.Conn {
	retConn, exists := c.vpnServerConnections[targetName]
	if exists {
		return retConn
	} else {
		return nil
	}
}

// getVPNClientConn gets all client jrpc connections for a target
func (c *ConnectionManager) getVPNClientConn(targetName string) []*jsonrpc2.Conn {
	retConns, exists := c.vpnClientConnections[targetName]
	if exists {
		return retConns
	} else {
		return []*jsonrpc2.Conn{}
	}
}
