package main

import (
	"database/sql"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"inet.af/netaddr"
)

/* Create a target */
func (a *app) createTarget(c *fiber.Ctx) error {
	authUser := c.Locals("user")
	if authUser == nil {
		return c.Status(401).JSON(ErrorResponse("unauthorized"))
	}

	target := c.Params("target")
	if target == "" {
		return c.Status(400).JSON(ErrorResponse("target must not be empty"))
	}

	res, err := a.db.ExecContext(c.Context(), "INSERT INTO targets VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11) ON CONFLICT DO NOTHING",
		target, authUser, "10.8.0.0", "/23", "1.1.1.1", "", "", "", "10.8.0.1", "/23", "")
	if err != nil {
		a.log.Error("something went wrong with database query", zap.Error(err))
		return c.Status(500).JSON(ErrorResponse("something went wrong"))
	}

	numRowsAffected, err := res.RowsAffected()
	if err != nil {
		a.log.Error("something went wrong with database query", zap.Error(err))
		return c.Status(500).JSON(ErrorResponse("something went wrong"))
	}

	if numRowsAffected == 1 {
		return c.Status(200).SendString("successfully created target")
	} else {
		return c.Status(400).JSON(ErrorResponse("target already exists"))
	}
}

/* Returns available connection targets, this is NOT exhaustive */
func (a *app) getTargets(c *fiber.Ctx) error {
	authUser := c.Locals("user")
	if authUser == nil {
		return c.Status(401).JSON(ErrorResponse("unauthorized"))
	}

	rows, err := a.db.QueryContext(c.Context(), "SELECT target FROM target_acl WHERE principal=$1", authUser)
	if err != nil {
		a.log.Error("something went wrong with database query", zap.Error(err))
		return c.Status(500).JSON(ErrorResponse("something went wrong"))
	}
	defer rows.Close()

	ret := make(ListTargetsResponse, 0)
	for rows.Next() {
		var target string
		err := rows.Scan(&target)
		if err != nil {
			a.log.Error("failed to parse sql row", zap.Error(err))
			return c.Status(500).JSON(ErrorResponse("something went wrong"))
		}

		formattedTarget := struct {
			Name string `json:"name"`
		}{
			Name: target,
		}
		ret = append(ret, formattedTarget)
	}

	// Also return targets where user is the owner
	rows, err = a.db.QueryContext(c.Context(), "SELECT name FROM targets WHERE owner=$1", authUser)
	if err != nil {
		a.log.Error("something went wrong with database query", zap.Error(err))
		return c.Status(500).JSON(ErrorResponse("something went wrong"))
	}
	defer rows.Close()

	var target string
	for rows.Next() {
		err := rows.Scan(&target)
		if err != nil {
			a.log.Error("failed to parse sql row", zap.Error(err))
			return c.Status(500).JSON(ErrorResponse("something went wrong"))
		}

		formattedTarget := struct {
			Name string `json:"name"`
		}{
			Name: target,
		}
		ret = append(ret, formattedTarget)
	}

	return c.Status(200).JSON(ret)
}

/* Creates a new connection using provided name and pubkey */
func (a *app) createConnection(c *fiber.Ctx) error {
	authUser := c.Locals("user")
	if authUser == nil {
		return c.Status(401).JSON(ErrorResponse("unauthorized"))
	}

	target := c.Params("target")
	if target == "" {
		return c.Status(400).JSON(ErrorResponse("target must not be empty"))
	}

	reqBody := NewConnectionRequest{}
	if err := c.BodyParser(&reqBody); err != nil {
		return err
	}

	if reqBody.Name == "" || reqBody.Pubkey == "" {
		return c.Status(400).JSON(ErrorResponse("name and pubkey must not be empty"))
	}

	var serverPubkey, serverPubIp, serverPubVpnPort, serverInternalIp, serverInternalCidr, dnsIp, serverHeartbeat string

	err := a.db.QueryRowContext(c.Context(), "SELECT server_pubkey, server_public_ip, server_public_vpn_port, server_internal_ip, server_internal_cidr, dns_ip, server_heartbeat FROM targets WHERE name=$1", target).Scan(&serverPubkey, &serverPubIp, &serverPubVpnPort, &serverInternalIp, &serverInternalCidr, &dnsIp, &serverHeartbeat)
	if err == sql.ErrNoRows {
		return c.Status(400).JSON(ErrorResponse("target not found"))
	}

	// perform liveness probe of the server
	if serverPubkey == "" || serverPubIp == "" || serverPubVpnPort == "" {
		return c.Status(503).JSON(ErrorResponse("server is not currently available"))
	}

	var targetId string
	err = a.db.QueryRowContext(c.Context(), "SELECT id FROM connections WHERE target=$1 AND pubkey=$2", target, reqBody.Pubkey).Scan(&targetId)
	if err != sql.ErrNoRows {
		return c.Status(400).JSON(ErrorResponse("connection already exists"))
	}

	// we create a new connection for this device using the public key
	rows, err := a.db.QueryContext(c.Context(), "SELECT client_ip FROM connections WHERE target=$1", target)
	if err != nil {
		a.log.Error("something went wrong with database query", zap.Error(err))
		return c.Status(500).JSON(ErrorResponse("something went wrong"))
	}
	defer rows.Close()

	var clientIp string
	clientIpSet := make(map[string]struct{})
	for rows.Next() {
		err := rows.Scan(&clientIp)
		if err != nil {
			a.log.Error("failed to parse sql row", zap.Error(err))
			return c.Status(500).JSON(ErrorResponse("something went wrong"))
		}
		clientIpSet[clientIp] = struct{}{}
	}

	serverIpPrefix, err := netaddr.ParseIPPrefix(serverInternalIp + serverInternalCidr)
	if err != nil {
		a.log.Error("failed to parse server ip prefix", zap.Error(err))
		return c.Status(500).JSON(ErrorResponse("something went wrong"))
	}

	serverIpRange := serverIpPrefix.Range()
	currIp := serverIpPrefix.IP().Next() // iterate past the first ip because it is reserved for the server
	maxIp := serverIpRange.To()

	var ipToAllocate string

	for !maxIp.Less(currIp) {
		// iterate while currIp is less than or equal to maxIp
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
		a.log.Error("no available ips for target" + target)
		return c.Status(503).JSON(ErrorResponse("no available ips, retry at a later time"))
	}

	// we found an ip to allocate, use a transaction to ensure this is not raced
	tx, err := a.db.BeginTx(c.Context(), nil)
	if err != nil {
		a.log.Error("failed to begin transaction", zap.Error(err))
		return c.Status(500).JSON(ErrorResponse("something went wrong"))
	}
	defer tx.Rollback()

	err = tx.QueryRowContext(c.Context(), "SELECT id FROM connections WHERE target=$1 AND client_ip=$2", target, ipToAllocate).Scan(&targetId)
	if err != sql.ErrNoRows {
		a.log.Error("failed to acquire ip in transaction", zap.Error(err))
		return c.Status(503).JSON(ErrorResponse("failed to acquire ip, retry at a later time"))
	}

	connectionId := uuid.New().String()
	_, err = tx.ExecContext(c.Context(), "INSERT INTO connections VALUES ($1, $2, $3, $4, $5, $6)", connectionId, target, reqBody.Name, reqBody.Pubkey, ipToAllocate, serverInternalCidr)
	if err != nil {
		a.log.Error("failed to insert new connection", zap.Error(err))
		return c.Status(500).JSON(ErrorResponse("something went wrong"))
	}

	err = tx.Commit()
	if err != nil {
		a.log.Error("something went wrong while commiting", zap.Error(err))
		return c.Status(500).JSON(ErrorResponse("something went wrong"))
	}

	responseObj := NewConnectionResponse{
		Config: &ConnectionConfig{
			PublicKey:  &serverPubkey,
			ClientIp:   &ipToAllocate,
			ClientCidr: &serverInternalCidr,
			ServerIp:   &serverPubIp,
			ServerPort: &serverPubVpnPort,
			DnsIp:      &dnsIp,
		},
		Id: &connectionId,
	}
	return c.Status(200).JSON(responseObj)
}
