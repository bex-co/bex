-- Least-privilege control-plane grants for the ssh-gateway Postgres role
-- (w7/m56, docs/ADR035-ssh.md §116). This is the SINGLE SOURCE of the gateway's
-- database privilege surface: scripts/ssh-gateway-db-role.sh applies it in
-- production and dbrole_integration_test.go applies the identical text in CI, so
-- the shipped boundary and the tested boundary cannot drift.
--
-- The role must already exist (the script/test create it and set its password).
-- __ROLE__ is substituted with the concrete role name by the caller. Every grant
-- below maps to a method the gateway actually calls — nothing more is granted, so
-- a stolen gateway credential cannot read api-key/OAuth, billing, usage,
-- registry-credential, git-connection, sandbox-key, or app/domain rows: those
-- tables are simply never granted (a fresh role has no table access by default).

-- Referencing public.* tables requires schema USAGE.
GRANT USAGE ON SCHEMA public TO __ROLE__;

-- Authenticate the SSH caller and resolve their workspace + membership. These are
-- READ-ONLY and are what the SSH authorization path (apps.ResolveSSHSession ->
-- AuthorizeApp -> the workspace resolver) needs; the gateway cannot write them.
GRANT SELECT ON ssh_keys TO __ROLE__;
GRANT SELECT ON tenants TO __ROLE__;
GRANT SELECT ON tenant_members TO __ROLE__;

-- Its OWN session-audit rows: StartSSHSession INSERTs, EndSSHSession UPDATEs
-- (whose WHERE also needs SELECT on this one table).
GRANT SELECT, INSERT, UPDATE ON ssh_sessions TO __ROLE__;

-- The cross-replica exec-ticket single-use claim: ClaimShellNonce DELETEs expired
-- rows (WHERE needs SELECT) and INSERTs the claimed nonce.
GRANT SELECT, INSERT, DELETE ON shell_ticket_nonces TO __ROLE__;

-- The authorization audit trail (core.Base.Audit -> PGStore.Record). Append-only:
-- INSERT alone, no read of others' audit history.
GRANT INSERT ON audit_events TO __ROLE__;
