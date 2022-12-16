package main

import (
	"context"
	"database/sql"
)

// RVPNDatabase represents a rVPN database
type RVPNDatabase struct {
	db *sql.DB
}

// RVPNTarget represents a rVPN target
type RVPNTarget struct {
	name                string
	owner               string
	networkIp           string
	networkCidr         string
	dnsIp               string
	serverPubkey        string
	serverPublicIp      string
	serverPublicVpnPort string
	serverInternalIp    string
	serverInternalCidr  string
	serverHeartbeat     string
}

// RVPNConnection represents a connect to the rVPN control plane
type RVPNConnection struct {
	id         string
	target     string
	deviceId   string
	pubkey     string
	clientIp   string
	clientCidr string
}

func NewRVPNDatabase(postgresURL string) (*RVPNDatabase, error) {
	db, err := sql.Open("postgres", postgresURL)
	if err != nil {
		return nil, err
	}

	return &RVPNDatabase{
		db: db,
	}, nil
}

// createTarget creates a target, returns whether it was created or not
func (d *RVPNDatabase) createTarget(ctx context.Context, name, owner, networkIp, networkCidr, dnsIp, serverPubkey, serverPublicIp, serverPublicVpnPort, serverInternalIp, serverInternalCidr, serverHeartbeat string) (bool, error) {
	res, err := d.db.ExecContext(ctx, "INSERT INTO targets (name, owner, network_ip, network_cidr, dns_ip, server_pubkey, server_public_ip, server_public_vpn_port, server_internal_ip, server_internal_cidr, server_heartbeat) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11) ON CONFLICT DO NOTHING",
		name, owner, networkIp, networkCidr, dnsIp, serverPubkey, serverPublicIp, serverPublicVpnPort, serverInternalIp, serverInternalCidr, serverHeartbeat)
	if err != nil {
		return false, err
	}

	numRowsAffected, err := res.RowsAffected()
	if err != nil {
		return false, err
	}

	return numRowsAffected == 1, nil
}

// getTargetsByPrincipal gets targets principal is authorized to access by ACL rules
func (d *RVPNDatabase) getTargetsByPrincipal(ctx context.Context, principal string) ([]string, error) {
	rows, err := d.db.QueryContext(ctx, "SELECT target FROM target_acl WHERE principal=$1", principal)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var target string
	ret := make([]string, 0, 5)

	for rows.Next() {
		err := rows.Scan(&target)
		if err != nil {
			return nil, err
		}

		ret = append(ret, target)
	}

	return ret, nil
}

// getTargetsByOwner gets targets where owner is the owner
func (d *RVPNDatabase) getTargetsByOwner(ctx context.Context, owner string) ([]string, error) {
	rows, err := d.db.QueryContext(ctx, "SELECT name FROM targets WHERE owner=$1", owner)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var target string
	ret := make([]string, 0, 5)

	for rows.Next() {
		err := rows.Scan(&target)
		if err != nil {
			return nil, err
		}

		ret = append(ret, target)
	}

	return ret, nil
}

// createDevice creates a device and returns whether or not the device was created or already existed
func (d *RVPNDatabase) createDevice(ctx context.Context, principal, hardwareId, deviceId string) (bool, error) {
	res, err := d.db.ExecContext(ctx, "INSERT INTO devices (principal, hardware_id, device_id) VALUES ($1, $2, $3) ON CONFLICT DO NOTHING",
		principal, hardwareId, deviceId)
	if err != nil {
		return false, err
	}

	numRowsAffected, err := res.RowsAffected()
	if err != nil {
		return false, err
	}

	return numRowsAffected == 1, nil
}

// getDeviceId gets a device id from principal and hardware id
func (d *RVPNDatabase) getDeviceId(ctx context.Context, principal, hardwareId string) (string, error) {
	row := d.db.QueryRowContext(ctx, "SELECT device_id FROM devices WHERE principal=$1 AND hardware_id=$2", principal, hardwareId)

	var deviceId string
	err := row.Scan(&deviceId)
	if err != nil {
		if err == sql.ErrNoRows {
			// no rows, return empty string
			return "", nil
		} else {
			// actual database error
			return "", err
		}
	}

	return deviceId, nil
}

// getTargetByName gets a target by the name of the target which is the primary key
func (d *RVPNDatabase) getTargetByName(ctx context.Context, target string) (*RVPNTarget, error) {
	row := d.db.QueryRowContext(ctx, `
		SELECT 
			name, owner, network_ip, network_cidr, dns_ip, server_pubkey, server_public_ip, server_public_vpn_port, server_internal_ip, server_internal_cidr, server_heartbeat
		FROM targets
		WHERE name=$1
	`, target)

	retRVPNTarget := RVPNTarget{}
	err := row.Scan(&retRVPNTarget.name, &retRVPNTarget.owner, &retRVPNTarget.networkIp, &retRVPNTarget.networkCidr, &retRVPNTarget.dnsIp, &retRVPNTarget.serverPubkey,
		&retRVPNTarget.serverPublicIp, &retRVPNTarget.serverPublicVpnPort, &retRVPNTarget.serverInternalIp, &retRVPNTarget.serverInternalCidr, &retRVPNTarget.serverHeartbeat)
	if err != nil {
		if err == sql.ErrNoRows {
			// no rows, return nil
			return nil, nil
		} else {
			// actual database error
			return nil, err
		}
	}

	return &retRVPNTarget, nil
}

func (d *RVPNDatabase) getTargetClientIps(ctx context.Context, target string) (map[string]struct{}, error) {
	rows, err := d.db.QueryContext(ctx, `
		SELECT client_ip FROM connections WHERE target=$1
	`, target)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var clientIp string
	clientIpSet := make(map[string]struct{})
	for rows.Next() {
		err := rows.Scan(&clientIp)
		if err != nil {
			return nil, err
		}
		clientIpSet[clientIp] = struct{}{}
	}

	return clientIpSet, nil
}

// getConnection gets a connection if it exists otherwise it returns the default struct
func (d *RVPNDatabase) getConnection(ctx context.Context, targetName, deviceId string) (RVPNConnection, error) {
	row := d.db.QueryRowContext(ctx, `
		SELECT
			id, target, device_id, pubkey, client_ip, client_cidr 
		FROM connections
		WHERE target=$1 AND pubkey=$2
	`, targetName, deviceId)

	retRVPNConnection := RVPNConnection{}
	err := row.Scan(&retRVPNConnection.id, &retRVPNConnection.target, &retRVPNConnection.deviceId,
		&retRVPNConnection.pubkey, &retRVPNConnection.clientIp, &retRVPNConnection.clientCidr)
	if err != nil {
		if err == sql.ErrNoRows {
			// no rows, return default RVPNConnection struct
			return retRVPNConnection, nil
		} else {
			return retRVPNConnection, err
		}
	}

	return retRVPNConnection, nil
}

// createConnection creates a a connection from rVPN client to rVPN server and returns whether it was created or already existed
func (d *RVPNDatabase) createConnection(ctx context.Context, id, target, deviceId, pubkey, clientIp, clientCidr string) (bool, error) {
	res, err := d.db.ExecContext(ctx, `
		INSERT INTO connections (id, target, device_id, pubkey, client_ip, client_cidr) 
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT DO NOTHING
	`, id, target, deviceId, pubkey, clientIp, clientCidr)
	if err != nil {
		return false, err
	}

	numRowsAffected, err := res.RowsAffected()
	if err != nil {
		return false, err
	}

	return numRowsAffected == 1, nil
}

// updateConnection updates a a connection from rVPN client to rVPN server and returns whether it was a row was affected
func (d *RVPNDatabase) updateConnection(ctx context.Context, id, target, deviceId, pubkey, clientIp, clientCidr string) (bool, error) {
	res, err := d.db.ExecContext(ctx, `
		UPDATE connections
		SET target=$2, device_id=$3, pubkey=$4, client_ip=$5, client_cidr=$6
		WHERE id=$1
	`, id, target, deviceId, pubkey, clientIp, clientCidr)
	if err != nil {
		return false, err
	}

	numRowsAffected, err := res.RowsAffected()
	if err != nil {
		return false, err
	}

	return numRowsAffected == 1, nil
}
