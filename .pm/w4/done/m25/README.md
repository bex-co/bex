# w4 · m25 — Identity completeness: user `name` + machine-caller resolution

**Worker:** worker4 **Goal:** `render whoami` and every place bex surfaces a user (owners, members, dashboard) return a real name and email for both session-derived and API-key callers, closing the two recorded identity blanks. **Status:** done 2026-07-15

## Tasks (in order)

| id   | title                                                              | est | depends_on |
| ---- | ------------------------------------------------------------------ | --- | ---------- |
| t001 | Kratos identity schema: `name` trait + settings/registration — **DONE** | 45m | —          |
| t002 | Thread name through CurrentUser/owners/members (REST/GraphQL/MCP) — **DONE** | 30m | t001       |
| t003 | API-key caller → identity email/name resolution — **DONE**         | 45m | t001       |
| t004 | Dashboard profile display + checklist `whoami` re-verify — **DONE** | 30m | t002, t003 |
| t005 | Render parity — **DONE**                                           | 30m | t004       |
| t006 | Simplify — **DONE**                                                | 30m | t005       |
| t007 | Test coverage — **DONE**                                           | 45m | t005       |
| t008 | Closeout — **DONE**                                                | 15m | t007       |

## Outcome (2026-07-15)

- Kratos schema (gitops base values, layered by every dev-N harness) gained an optional `name` trait; existing identities keep authenticating (verified live with a pre-trait identity) and can add a name via the Ory-Elements-rendered settings flow (verified via the API settings flow and the dashboard).
- `core.Identity` carries `Name` (session traits) and `ClientID` (introspection stamp), so `CurrentUser` branches on typed data: session/user-OAuth subjects resolve directly in Kratos, client_credentials keys resolve through the `bex.co/created-by` binding (`APIKeyStore.KeyOwner`, wired structurally as `workspaces.KeyOwnerReader` — no adapter). Unowned keys keep the earliest-admin-email fallback with an honest empty name.
- Members REST (`teamMember.name`) populated; GraphQL/MCP deliberately unchanged (Render's captured dashboard GraphQL and official MCP carry no user name). Dashboard user menu + Settings → Profile show/edit the name.
- Social login maps GitHub's `name` claim at the single Jsonnet ingestion point (`scripts/auth-secrets.sh`).
- Live DoD evidence: `GET /v1/users` → `{"email":"ada@dev4.test","name":"Ada Lovelace"}` for a session token **and** a client_credentials token against dev-4; official CLI `render whoami` prints `Name: Ada Lovelace / Email: ada@dev4.test`; screenshot `.playwright-mcp/m25-user-nav-name.png`. `docs/cli-compatibility-checklist.md` whoami row and `docs/ADR018-render-parity.md` residual updated.
- Follow-up filed: `w4/020` (`mfaEnabled` false positive — Kratos mints a webauthn stub at password-only registration; pre-existing, found by this milestone's live verify).

## Definition of done

`GET /v1/users` returns a populated `name` and `email` for a session caller **and** an API-key caller (official CLI `render whoami` shows both live); the "Name … is always \"\"" comment at `lego/backend/internal/workspaces/service.go:285` and `docs/ADR018-render-parity.md:217`'s machine-caller residual are gone, recorded with evidence.

## Source + Goal linkage

- **Source:** `/pm-brainstorm` round 12, 2026-07-15 — code miner (`workspaces/service.go:285`) + ADR018:217's left-open machine-caller half (w4/016 closed only the session-caller case).
- **Goal linkage:** Render user-object parity; w4's identity mandate (Kratos owns the identity model).
- **Expected outcome:** no blank identity fields anywhere bex shows a user.
- **Why now:** continues the m20/m23 identity-payload thread while it's warm; w4's open milestones are nearly closed. Render parity closing task included — REST/GraphQL/MCP/UI surface change.
