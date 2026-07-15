# w4 · m25 — Identity completeness: user `name` + machine-caller resolution

**Worker:** worker4 **Goal:** `render whoami` and every place bex surfaces a user (owners, members, dashboard) return a real name and email for both session-derived and API-key callers, closing the two recorded identity blanks. **Status:** todo

## Tasks (in order)

| id   | title                                                            | est | depends_on   |
| ---- | ---------------------------------------------------------------- | --- | ------------ |
| t001 | Kratos identity schema: `name` trait + settings/registration     | 45m | —            |
| t002 | Thread name through CurrentUser/owners/members (REST/GraphQL/MCP) | 30m | t001         |
| t003 | API-key caller → identity email/name resolution                  | 45m | t001         |
| t004 | Dashboard profile display + checklist `whoami` re-verify         | 30m | t002, t003   |
| t005 | Render parity                                                    | 30m | t004         |
| t006 | Simplify                                                         | 30m | t005         |
| t007 | Test coverage                                                    | 45m | t005         |
| t008 | Closeout                                                         | 15m | t007         |

## Definition of done

`GET /v1/users` returns a populated `name` and `email` for a session caller **and** an API-key caller (official CLI `render whoami` shows both live); the "Name … is always \"\"" comment at `lego/backend/internal/workspaces/service.go:285` and `docs/ADR018-render-parity.md:217`'s machine-caller residual are gone, recorded with evidence.

## Source + Goal linkage

- **Source:** `/pm-brainstorm` round 12, 2026-07-15 — code miner (`workspaces/service.go:285`) + ADR018:217's left-open machine-caller half (w4/016 closed only the session-caller case).
- **Goal linkage:** Render user-object parity; w4's identity mandate (Kratos owns the identity model).
- **Expected outcome:** no blank identity fields anywhere bex shows a user.
- **Why now:** continues the m20/m23 identity-payload thread while it's warm; w4's open milestones are nearly closed. Render parity closing task included — REST/GraphQL/MCP/UI surface change.
