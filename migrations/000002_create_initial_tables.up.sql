-- targets are the basic unit of rVPN and represent a VPN server

CREATE TABLE targets (
    name VARCHAR ( 50 ) PRIMARY KEY,
    owner VARCHAR ( 50 ) NOT NULL,
    network_ip VARCHAR ( 16 ) NOT NULL,
    network_cidr VARCHAR ( 5 ) NOT NULL,
    dns_ip VARCHAR ( 16 ) NOT NULL,
    server_public_ip VARCHAR ( 16 ),
    server_public_vpn_port VARCHAR ( 5 ),
    server_internal_ip VARCHAR ( 16 ),
    server_internal_cidr VARCHAR ( 5 )
);

-- acls which control access to a target, allow only

CREATE TABLE target_acl (
    principal VARCHAR ( 50 ),
    target VARCHAR ( 50 )
);

-- each connection identifies a device

CREATE TABLE connections (
    id VARCHAR ( 50 ),
    target VARCHAR ( 50 ),
    name VARCHAR ( 50 ),
    pubkey VARCHAR ( 50 ),
    client_ip VARCHAR ( 16 ),
    client_cidr VARCHAR ( 5 )
);
