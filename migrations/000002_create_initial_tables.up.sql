-- targets are the basic unit of rVPN and represent a VPN server

CREATE TABLE targets (
    name VARCHAR ( 50 ) PRIMARY KEY,
    owner VARCHAR ( 50 ) NOT NULL,
    serer_ip VARCHAR ( 16 ),
    server_vpn_port VARCHAR ( 5 )
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
    pubkey VARCHAR ( 50 )
);
