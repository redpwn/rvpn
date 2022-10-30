package main

/*

// Creates a new connection using provided name and pubkey
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

	// perform liveness probe of the server; TODO: check heartbeat to ensure that last heartbeat was within 10 minutes
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

	// we found an ip to allocate, ensure this is not raced via unique db constraint
	connectionId := uuid.New().String()
	_, err = a.db.ExecContext(c.Context(), "INSERT INTO connections VALUES ($1, $2, $3, $4, $5, $6)", connectionId, target, reqBody.Name, reqBody.Pubkey, ipToAllocate, serverInternalCidr)
	if err != nil {
		a.log.Error("failed to insert new connection", zap.Error(err))
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

*/
