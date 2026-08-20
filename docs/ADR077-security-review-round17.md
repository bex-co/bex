# ADR077: Security review round 17 — geyRc8 disposition

- **Status**: Accepted (2026-08-19)
- **Scan**: codex-security `geyRc8`, repository revision `e3562e563f` (reviewed against HEAD after ADR076), 15 findings (7 medium, 8 low)
- **Lineage**: seventeenth pass in the ADR028 → … → ADR073 → ADR076 lineage

## Summary

Eleven findings are fixed in place with regression tests. Finding 5 (Web Push SSRF) was already closed in ADR076 #6 and is re-confirmed. Finding 4 (`sslmode=require`) stays the ADR009 public-Postgres contract. Finding 8 is the standing onbex.co PSL residual (twelfth report). Finding 10 remains the ADR060 §D7 digest-pinning inventory (installer Sigstore + export/age pins already closed in ADR076).

| # | Finding | Severity | Disposition |
| --- | --- | --- | --- |
| 1 | Registry credentials unbounded | medium | **Fixed** — `BEX_MAX_REGISTRY_CREDS_PER_WORKSPACE` (default 50) + field-size caps; `REGISTRY_CREDENTIAL_LIMIT` |
| 2 | OAuth revoke leaves warm cache across replicas | medium | **Fixed** — durable `oauth_revocations` markers; dashboard posts `POST /v1/oauth/revocations`; API-key mint introspects fresh |
| 3 | Serial Web Push starves shared queue | medium | **Fixed** — bounded concurrent send pool (`pushSendConcurrency=8`) |
| 4 | Public Postgres `sslmode=require` | medium | **Accepted residual** — ADR009 contract; TLS encrypts without hostname verification until a public-host cert lifecycle ships |
| 5 | Web Push SSRF / redirects | medium | **Already fixed** (ADR076 #6) — `SafeDialContext`, no proxy, no redirects; HTTPS-only registration |
| 6 | Cleartext `http://` Git URLs | medium | **Fixed** — `ValidRepo` + CRD pattern require `https://`/`ssh://`/`git@` |
| 7 | Git credential proxy missing budgets | medium | **Fixed** — `MaxDuration`, response byte cap, cumulative session/workspace request budgets |
| 8 | Shared onbex.co tenant suffix | low | **Accepted residual** — onbex.co PSL (twelfth report); `.pm/DO_NOT_DO.md` `#PSL` |
| 9 | Agent attach unaudited redemption | low | **Fixed** — durable `StartSSHSession`/`EndSSHSession` on ticket redeem |
| 10 | Digest-pinning inventory | low | **Deferred** — ADR060 §D7; installer/export/age pieces already closed in ADR076 |
| 11 | Native SSH audit wrong instance | low | **Fixed** — sticky connection target; audit InstanceID matches exec |
| 12 | Workspace delete leaves `git_connections` | low | **Fixed** — FK `ON DELETE CASCADE` (migration `0094`) |
| 13 | Browser shell audits Traefik peer | low | **Fixed** — trusted-proxy XFF recovery on webshell |
| 14 | API-key quota race across replicas | low | **Fixed** — tenant advisory lock around count→create→bind |
| 15 | Registry credential OpenBao orphans | low | **Fixed** — OpenBao-first fail-closed delete + pre-cascade `WorkspacePurger` |

## Finding 1 (medium) — registry-credential quota

Create allocated Postgres metadata and OpenBao secrets with no per-workspace cap.

**Fix.** `CountRegistryCredentials` + `Service.MaxCredentials` (env `BEX_MAX_REGISTRY_CREDS_PER_WORKSPACE`, default 50, `0` disables). Over-quota creates refuse with coded `REGISTRY_CREDENTIAL_LIMIT`. Host/username/name ≤ 253 bytes and secret ≤ 64 KiB are rejected before either backend write.

## Finding 2 (medium) — shared OAuth revocation markers

Process-local `invalidate` could not clear a warm positive cache on the other production replica; dashboard Hydra revoke never notified bex-api.

**Fix.** Migration `0093` adds `oauth_revocations`. `invalidate` and session-authenticated `POST /v1/oauth/revocations` bump `(subject, client_id)`. Cache hits compare entry time against the durable marker. `POST /v1/api-keys` introspects fresh (bypass positive cache). Dashboard connected-agent revoke posts the marker after Hydra succeeds.

## Finding 3 (medium) — parallel Web Push send

The worker claimed up to 50 deliveries then sent them serially, so slow tenant endpoints held the lease.

**Fix.** `send` fans out through a semaphore of eight concurrent deliveries while preserving claim/complete semantics and `errors.Join` aggregation.

## Finding 4 (medium) — `sslmode=require` (accepted)

Generated public URLs use `sslmode=require` by design ([ADR009](ADR009-postgresql-management.md)). Emitting `verify-full` without a public-hostname certificate/CA lifecycle would break every generated client. Tracked as an accepted residual until that lifecycle ships.

## Finding 5 (medium) — Web Push SSRF (already fixed)

Re-confirmed ADR076 #6: production client uses `SafeDialContext`, `Proxy: nil`, and no redirects; registration is HTTPS-only.

## Finding 6 (medium) — refuse cleartext Git

**Fix.** `ValidRepo` / `repoRE` and the App CRD `spec.repo` pattern accept only `https://`, `ssh://`, and `git@`. Cleartext `http://` fails API validation and admission.

## Finding 7 (medium) — Git proxy budgets

**Fix.** Mirror the model-proxy pattern: per-exchange `MaxDuration` (default 10m), response body cap, and cumulative `BEX_AGENT_GIT_MAX_REQUESTS_PER_{SESSION|WORKSPACE}` charged after session identity and before mint.

## Finding 8 (low) — onbex.co PSL (twelfth report)

Unchanged. Do not unset `BEX_BASE_DOMAIN` or submit to the PSL yet (`.pm/DO_NOT_DO.md` `#PSL`). Operator tracking: `.pm/w1/050.md`.

## Finding 9 (low) — agent-attach audit

**Fix.** After nonce claim + fresh authorization, record content-free `StartSSHSession` / `EndSSHSession` with subject, session, action, and trusted client source.

## Finding 10 (low) — digest inventory (deferred)

Standing ADR060 §D7 residual. Installer Sigstore verification and CNPG-export / age pins landed in ADR076; gVisor node bootstrap, CNPG Cluster serving tags, and remaining Dockerfile/kpack/barman pins stay deferred.

## Finding 11 (low) — sticky SSH instance

**Fix.** Connection state holds the handshake-selected instance. Channel reauthorization resolves that sticky ID (fallback only if it is gone). Durable audit `InstanceID` matches the executed pod.

## Finding 12 (low) — cascade GitHub connections

**Fix.** Migration `0094` deletes orphan rows, then adds `git_connections.workspace_id REFERENCES tenants(id) ON DELETE CASCADE`, freeing installation IDs for reconnect.

## Finding 13 (low) — webshell client IP

**Fix.** When the immediate peer is in the gateway's trusted-proxy CIDRs, parse `X-Forwarded-For` defensively (same rightmost-untrusted walk as bex-api). Untrusted peers cannot spoof forwarded headers.

## Finding 14 (low) — API-key quota lock

**Fix.** Production `KeyBinder` implements `KeyQuotaLocker` via `pg_advisory_xact_lock(hashtext(tenant_id))` around count→create→bind. Process-local `createMu` remains a fast path.

## Finding 15 (low) — OpenBao registry-credential purge

**Fix.** Individual delete removes the OpenBao path first and fails closed on secret errors. `registrycreds.WorkspacePurger` runs pre-cascade so workspace deletion does not leave unreachable passwords.

## Deferred and follow-up

- **Public Postgres `verify-full`** (finding 4): public-host certificate + CA distribution lifecycle amending ADR009.
- **onbex.co PSL** (finding 8, twelfth report): `.pm/w1/050.md` — do not unset `BEX_BASE_DOMAIN`.
- **Digest-pinning inventory** (finding 10): ADR060 §D7.
