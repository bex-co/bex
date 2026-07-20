-- w1/042 L7 (deferred from w1/m53): web-shell exec-ticket single-use across
-- gateway replicas. The gateway kept consumed ticket nonces in a per-process
-- map, so with N replicas a ticket was redeemable once per replica within its
-- 90s TTL. This table is the shared claim: INSERT … ON CONFLICT DO NOTHING is
-- the atomic claim-on-first-use (exactly one replica wins), and expired rows
-- are pruned opportunistically on each claim (store.ClaimShellNonce) — the
-- table's steady-state size is the number of shells opened in the last ~90s.
CREATE TABLE shell_ticket_nonces (
    nonce      text PRIMARY KEY,
    expires_at timestamptz NOT NULL
);
