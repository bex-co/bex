# w9 · m450 — Every audit row says who acted and from where: actor email, source IP and user agent

**Worker:** worker9 **Goal:** REST audit rows carry Render's `actor.email` for a person and `source_ip`/`user_agent` metadata when known; GraphQL, the dashboard and the CSV export show the same, and the CSV `actor` column is the email as on Render **Status:** blocked (t001–t007 done; t008 waits on the production pin carrying the w9/m450 commit)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Record the caller's source IP and user agent on every audit row — **DONE** | 1h | — |
| [t002](done/t002.md) | REST answers actor.email and the source_ip and user_agent metadata — **DONE** | 45m | t001 |
| [t003](done/t003.md) | GraphQL, the dashboard row and the CSV export show who and from where — **DONE** | 1h | t002 |
| [t004](done/t004.md) | Amend ADR006, ADR018 and ADR086 for the new fields — **DONE** | 30m | t003 |
| [t005](done/t005.md) | Render parity — **DONE** | 30m | t004 |
| [t006](done/t006.md) | Simplify — **DONE** | 20m | t005 |
| [t007](done/t007.md) | Test coverage — **DONE** | 30m | t005 |
| [t008](t008.md) | Closeout | 15m | t006, t007 |

## Definition of done

- On production, as the QA user in a Pro workspace, revealing a service's env vars in the dashboard records a row whose REST form has `actor: {type: "user", id: "usr-…", email: <the QA user's email>}`, `metadata.source_ip` equal to the client's public IP (not a Cloudflare, Traefik or node address) and `metadata.user_agent` equal to the browser's User-Agent.
- An API-key call's row carries `source_ip` and `user_agent` and no `email`; a system row carries neither.
- A request from an untrusted peer that sends `X-Forwarded-For` does not get that value as `source_ip` (unit test).
- The CSV export's `actor` column holds the email (a deleted user's `usr-` id), and its metadata includes `source_ip`/`user_agent` when present.

## Source + Goal linkage

- **Source:** `/pm-brainstorm one aspect of render parity to w9` (2026-11-16), filed by `/pmx them all to w9`. Evidence:
  - [render.com/docs/audit-logs](https://render.com/docs/audit-logs) (read 2026-11-16): "`source_ip` and `user_agent` metadata for audit events when available" (added 2026-02-19); CSV columns include "`actor` (user email, or a deleted user's ID)". Render changelog 2026-03-25: "Audit log updates: Added the EndShellEvent type, plus metadata for source IP and user agent".
  - The pinned `auditLogActor` (`lego/backend/internal/api/openapi/render-public-api-1.json`) has an optional `email`.
  - bex: `audit_events` has no IP or user-agent column; `renderAuditActor` omits email by design (`lego/backend/internal/audit/rest.go`); the CSV writes the raw subject (`dashboard/src/features/audit/lib/audit-csv.ts`).
- **Goal linkage:** [ADR008 § Render parity first](../../../docs/ADR008-vision.md#render-parity-first); a security investigation needs to know who acted and from where.
- **Expected outcome:** each audit row names the person by email and the network and client they acted from, on every surface and in the export, as Render's does.
- **Why now:** w9/m451 (shell events) and w9/m452 (sign-ins) both need these two columns, so adding them first avoids a second migration; Render has shipped them since 2026-02.
- **Decision state:** resolving the email at read time while storage keeps only the subject is the `/pm-brainstorm` **recommendation**: it keeps ADR006's redaction-by-structure in storage and amends only its "email omitted" sentence, an older design that conflicts with parity (AGENTS.md rule). What account deletion does to `source_ip`/`user_agent` (t004) is also a recommendation (follow ADR086's existing rule, let the 90-day retention purge); neither is a user decision.
- **Render parity task:** included. REST, GraphQL, the dashboard and the CSV export change.

## Progress

- **2026-11-16, t001–t007 (worker9, `/loopx w9`):**
  - **Recorded.** Migration `0283_audit_source_ip_user_agent` adds nullable `audit_events.source_ip`/`user_agent`. `api.RequestOriginMiddleware` (`cmd/api/main.go`) puts `core.TrustedProxies.SourceIP` (the edge's `True-Client-IP`, believed only from a `BEX_TRUSTED_PROXY_CIDRS` peer, else `ClientIP`) and the `User-Agent` on every `:8090` request; `core.attachRequestOrigin`, on both sink paths (`RecordAuditEvent`, `recordAudits`), copies them onto a row whose caller is that request's identity, the agent through `core.AuditUserAgent` (valid UTF-8, control characters removed, at most 512 bytes at a rune boundary: an invalid byte would have made Postgres refuse the row, which the sink swallows). System rows, rows naming someone else (the Stripe portal webhook), rows outside a request and the `:8091` listener store neither. The dashboard's SSR queries forward the browser request's `True-Client-IP` and `User-Agent` (`ssrForwardedHeaders`), so an SSR read's row names the person, not the pod.
  - **Answered.** REST `actor.email` (resolved at read time through Kratos' batched `LookupEmails`, `audit.Service.Emails`; best effort, never failing the log) and `metadata.source_ip`/`user_agent`; GraphQL `actorId`, `actorEmail`, `sourceIp`, `userAgent`; the dashboard table's new Source column (address, the client in short, full agent on hover; skeleton gained the matching two-line cell) and the CSV `actor` = email, else the id, with `source_ip`/`user_agent` in its metadata. MCP has no audit-log tool, as Render's has none.
  - **Docs.** ADR006 § Audit log amended (email resolved at read time; storage keeps only the subject), ADR018's Audit logs row, ADR086's table (`source_ip`/`user_agent` retained for the audit retention window and purged with the row, as `ssh_sessions.remote_address`).
  - **Parity (t005).** REST and GraphQL agree on the same rows (`TestAuditSurfaceParity`, through the pinned-schema validator); Render's keys are `source_ip`/`user_agent`. Drift filed as w9/099: Render's export names a deleted user `Deleted User ID#<id>`, bex the bare `usr-` id.
  - **Simplify (t006).** `Event.person()`/`actorID()` shared by both adapters.
  - **Tests (t007).** `TestSourceIPBehindProductionEdge`, `TestAuditUserAgentIsBoundedValidText`, `TestAttachRequestOriginOnlyForTheRequestersOwnRows`, `TestEveryAuditPathRecordsTheRequestOrigin` (core); `TestEverySurfaceRecordsWhereTheCallerActedFrom` (REST, GraphQL session and MCP through the composed middleware, plus an untrusted peer's spoofed headers); `TestRenderAuditLogUserActorsCarryTheirUsrIDAndEmail`, `TestResolveActorsKeepsTheLogWhenEmailsFail` (audit); `TestPGAuditRecordsWhereTheActorActedFrom` (real Postgres); dashboard `short-user-agent`, CSV, panel and SSR-header tests. Mutation spot-checks: believing `True-Client-IP` from any peer, or dropping the identity match, fails the core tests.
- **Gate (t008):** clears by itself once production runs the w9/m450 commit: `git log` of the `chore(deploy): pin platform images to <sha>` commit on `origin/main` must contain it. Then, as the QA user (`bash scripts/qa-login.sh`) in a Pro workspace, reveal a service's env vars in the dashboard and read `GET /v1/owners/{ownerId}/audit-logs`: the `ViewEnvVarValuesEvent` row must carry `actor.email`, `metadata.source_ip` (the client's public address, not a Cloudflare `172.64.0.0/13`-style edge, `10.x` pod or node address) and `metadata.user_agent`; an API-key call's row carries both and no email; export a CSV for the day and check its `actor` column and metadata.
