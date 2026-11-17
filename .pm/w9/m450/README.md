# w9 · m450 — Every audit row says who acted and from where: actor email, source IP and user agent

**Worker:** worker9 **Goal:** REST audit rows carry Render's `actor.email` for a person and `source_ip`/`user_agent` metadata when known; GraphQL, the dashboard and the CSV export show the same, and the CSV `actor` column is the email as on Render **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](t001.md) | Record the caller's source IP and user agent on every audit row | 1h | — |
| [t002](t002.md) | REST answers actor.email and the source_ip and user_agent metadata | 45m | t001 |
| [t003](t003.md) | GraphQL, the dashboard row and the CSV export show who and from where | 1h | t002 |
| [t004](t004.md) | Amend ADR006, ADR018 and ADR086 for the new fields | 30m | t003 |
| [t005](t005.md) | Render parity | 30m | t004 |
| [t006](t006.md) | Simplify | 20m | t005 |
| [t007](t007.md) | Test coverage | 30m | t005 |
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
