-- A deleted service's last egress hours are metered from the Traefik router
-- names and direct-egress flag its live egress meter last resolved (w6/m333;
-- ADR023): the App CR and its Ingresses go with the deletion, and the routers
-- key the HTTP egress counter. They are observed cluster state, so they live
-- beside the apps row rather than on it, and outlive it until the retired
-- record that needs them is pruned. No foreign key, for the same reason.
CREATE TABLE usage_egress_identities (
    service_id  text        PRIMARY KEY,
    routers     text[]      NOT NULL,
    direct      boolean     NOT NULL,
    recorded_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP
);
