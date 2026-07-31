# w7 · m56 — SSH gateway least-privilege DB credential

**Worker:** worker7 **Goal:** replace the SSH gateway's full control-plane Postgres credential with a narrowly scoped role (key lookup + session insert/update only), so a gateway compromise no longer yields the whole control-plane database. **Status:** done

## Tasks (in order)

| id   | title                                                                        | est | depends_on |            |
| ---- | ---------------------------------------------------------------------------- | --- | ---------- | ---------- |
| t001 | Define the scoped Postgres role + provisioning/rotation script               | 45m | —          | — **DONE** |
| t002 | Wire the gateway deployment to the scoped credential                         | 30m | t001       | — **DONE** |
| t003 | Negative verification + production rotation + ADR035 update                  | 45m | t002       | — **DONE** |
| t004 | Simplify                                                                     | 20m | t003       | — **DONE** |
| t005 | Test coverage: gateway store under the scoped role (allow + deny paths)      | 45m | t003       | — **DONE** |
| t006 | Closeout                                                                     | 15m | t005       | — **DONE** |

## Outcome (2026-07-30)

Shipped: the gateway connects as the scoped `bex_ssh_gateway` Postgres role (SELECT `ssh_keys`/`tenants`/`tenant_members`, INSERT/UPDATE `ssh_sessions`, SELECT/INSERT/DELETE `shell_ticket_nonces`, INSERT `audit_events` — nothing else), single-sourced from `internal/sshgateway/dbrole.sql` and consumed by both `scripts/ssh-gateway-db-role.sh` and the CI-durable `TestGatewayScopedRoleAllowsOwnSurfaceDeniesTheRest` (allow the full surface, deny sensitive tables with SQLSTATE 42501). The gateway stops running `Migrate`/`CheckOwnership` (bex-api owns the schema). Production role + secret provisioned and negatively verified live (`permission denied for table stripe_billing_events`/`registry_credentials`); the `config/ssh/deployment.yaml` credential swap is Argo-synced, revoking the gateway's mount of the full-privilege `bex-db-app`. **Three DoD assumptions corrected** (t003 evidence): the gateway legitimately needs tenant/membership reads (granted, not denied); there is no `api_keys` table (Hydra owns OAuth2 clients); and the gateway's `Migrate` needed a Go change, not just a credential swap.

## Definition of done

The production ssh-gateway connects with a Postgres role that can look up SSH public keys and insert/update its own session/audit rows and **nothing else** — negatively verified: with the gateway credential, SELECTs on tenant, API-key, and billing tables are denied by Postgres, not merely unexercised by Go code. The previous full-privilege grant to the gateway is revoked in production, the scoped credential has an install/rotate script following the existing out-of-band-secret pattern, and `docs/ADR035-ssh.md` §116's follow-up line is replaced with the shipped design plus rotation evidence.

## Source + Goal linkage

- **Source:** `docs/ADR035-ssh.md:116` — the recorded defense-in-depth follow-up ("replacing it with a separately granted lookup/insert/update-only role…"), surfaced by the 2026-07-30 `/pm-brainstorm for w7` docs sweep as the only unowned security follow-up left in `docs/`.
- **Goal linkage:** GOAL.md V0 #7 (security review) — blast-radius reduction on the platform's most exposed component: the gateway terminates untrusted public input on `:2222`, the browser-shell WebSocket, and the sandbox-exec listener.
- **Expected outcome:** a compromised gateway yields SSH-key lookup plus its own audit rows instead of every tenant's control-plane rows (API keys, billing, workspace data).
- **Why now:** the gateway's exposure grew twice since the follow-up was written (Browser Web Shell `w2/m55`, sandbox exec `w3/m33`) while its credential stayed full-privilege; the ADR028/m53 audit registers are otherwise empty, making this the last recorded-but-unowned item.
- **Render parity:** omitted — internal credential plumbing; no REST/GraphQL/MCP/UI surface change.
