# w5 · m384 — Round-24 hosting hardening: approve binds the plan it approves, a Grafana session ends soon after ops removal, a retired Postgres login's sessions end

**Worker:** worker5 **Goal:** three defense-in-depth gaps from round 24's hosting hunt close: a Blueprint approval applies only the plan its caller saw, a removed ops member loses obs.bex.co within the hour, and retiring a Postgres login (or restoring past its retirement) ends what it could still do. **Status:** blocked (t001, t002, t004–t006 done and shipped; t003 part done; t003's session termination waits on a privileged-path decision, and t002's deployment check on the deploy)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Blueprint approve binds the plan its caller saw (an optional sync id on REST, GraphQL and MCP; MCP passes it) — **DONE** | 45m | — |
| [t002](done/t002.md) | obs.bex.co: shorter Grafana sessions and an explicit insecure-email-lookup off; amend ADR088 §4 — **DONE** | 20m | — |
| [t003](t003.md) | A retired Postgres login's open sessions are terminated, and a point-in-time restore re-retires logins retired after its target | 1h30m | — |
| [t004](done/t004.md) | Render parity — **DONE** | 15m | t001, t002, t003 |
| [t005](done/t005.md) | Simplify — **DONE** | 15m | t004 |
| [t006](done/t006.md) | Test coverage — **DONE** | 30m | t004, t005 |
| [t007](t007.md) | Closeout | 15m | t006 |

Total: **7 tasks, about 3h40m**.

## Definition of done

1. Approving a Blueprint with the sync id of a plan that is no longer the newest planned sync answers 409 and applies nothing; without a sync id it behaves as today; the MCP `approve_blueprint` tool takes and passes the id the plan step returned.
2. Grafana's `login_maximum_lifetime_duration` and `login_maximum_inactive_lifetime_duration` are at most 1h, `oauth_allow_insecure_email_lookup` is set false explicitly, and ADR088 §4 states the new bound. A deployment check confirms the break-glass local login and basic auth are not reachable from the internet.
3. Retiring a managed Postgres login terminates its open sessions; a point-in-time restore whose target precedes a login's retirement retires it again before the restored database serves connections.
4. Each is covered by a regression test that fails on the pre-milestone code (the deployment check in DoD 2 excepted).

## Source + Goal linkage

- **Source:** inbox note [w5/190](../../done/190.md), from security review round 24 ([ADR107](../../../../docs/ADR107-security-review-round24.md), hosting hardening notes). Promoted by `/loopx w5` on 2026-11-13: three parts, together over an hour, and the Postgres part needs a privileged SQL path that does not exist yet.
- **Evidence:** `apps/blueprint_onboarding.go` `ApproveBlueprint` approves `LatestBlueprintPlan` with no binding; `deploy/gitops/overlays/prod/values/grafana.values.yaml` sets 24h/8h session lifetimes (ADR088 §4); the operator retires a login declaratively (`database_controller.go` `managedRoles`: NOLOGIN, NULL password), which does not end its open sessions, and `postgres/recovery.go` restores to a point that may precede a retirement.
- **Goal linkage:** Blueprints (ADR049), platform observability (ADR088), managed Postgres (ADR009); the security-review lineage (ADR107).
- **Expected outcome:** each of the three gaps is closed with a test.
- **Why now:** the remainder of round 24's hosting findings.
- **Render parity:** included (t004). The sync-id binding is an optional bex extension on Render-shaped approve surfaces; retiring a credential matches Render's "delete credential", whose sessions end.

## Findings (2026-11-13)

- **Decision needed for t003's session termination (security design, not taken unilaterally).** Ending a retired login's open sessions needs `pg_terminate_backend` from a role that may signal it, and nothing bex runs has one: the owner bex-api dials as holds only `pg_monitor`, and the operator has neither `pods/exec` nor CNPG's superuser access. The options:
  - **A.** Grant the owner `pg_signal_backend` and terminate from bex-api. Cheapest, but the tenant's owner gains the power to end any non-superuser backend in its cluster, including replication connections.
  - **B.** Give the operator `pods/exec` on database pods and run `psql` as `postgres` over the local socket. No tenant-visible power, but a new standing operator privilege.
  - **C.** A superuser-owned `SECURITY DEFINER` function that ends only bex-retired (NOLOGIN) roles' backends, installed by the operator in each database. Narrowest power, but needs an install path for existing clusters.
  - **Recommendation:** C, narrowest standing power; B if an install path for existing clusters is out of reach. Record the choice through `/unblock w5`.
- **Done in t003:** a recovered instance carries the source's retired logins (minus the owner, its default credential), so a login retired after the restore target is retired again once the restored primary starts.
- **Still open in t003:** a restored owner keeps its backup-time password, since the operator's recovery bootstrap sets neither `owner` nor `secret`. Check on a cluster whether setting them makes CNPG reset the owner's password after recovery (blocked locally by the w5/m370 inotify gate).
- **t002's deployment check:** after the next production deploy, confirm the Grafana break-glass login and basic auth are not reachable from the internet.

