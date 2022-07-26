-- targets are the basic unit of rVPN and represent a VPN server

CREATE TABLE targets (
    name VARCHAR PRIMARY KEY,
    owner VARCHAR NOT NULL,
    network_ip VARCHAR NOT NULL,
    network_cidr VARCHAR NOT NULL,
    dns_ip VARCHAR NOT NULL,
    server_pubkey VARCHAR,
    server_public_ip VARCHAR,
    server_public_vpn_port VARCHAR,
    server_internal_ip VARCHAR,
    server_internal_cidr VARCHAR,
    server_heartbeat VARCHAR
);

-- acls which control access to a target, allow only

CREATE TABLE target_acl (
    principal VARCHAR,
    target VARCHAR
);

-- each connection identifies a device

CREATE TABLE connections (
    id VARCHAR,
    target VARCHAR,
    name VARCHAR,
    pubkey VARCHAR,
    client_ip VARCHAR,
    client_cidr VARCHAR,
    UNIQUE (target, client_ip)
);
