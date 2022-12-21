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
    principal VARCHAR NOT NULL,
    target VARCHAR NOT NULL,
    access_type INTEGER
);

-- each connection is associated with a device

CREATE TABLE connections (
    id VARCHAR PRIMARY KEY,
    target VARCHAR NOT NULL,
    device_id VARCHAR NOT NULL,
    pubkey VARCHAR NOT NULL,
    client_ip VARCHAR,
    client_cidr VARCHAR,
    UNIQUE (target, client_ip)
);

-- devices are represented by a principal, hardware id (can be overridden by user, just used for uniqueness)

CREATE TABLE devices (
    principal VARCHAR,
    target VARCHAR,
    hardware_id VARCHAR,
    device_id VARCHAR,
    PRIMARY KEY (principal, target, hardware_id),
    UNIQUE (device_id)
)
