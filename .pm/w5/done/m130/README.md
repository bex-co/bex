# w5 · m130 — Not-found, name-conflict and unavailable refusals carry codes, and the dashboard branches on them

**Worker:** worker5 **Goal:** The refusals the dashboard still decides by wording carry stable codes on REST, GraphQL and MCP, and the dashboard decides by code, so rewording a bex-api message no longer changes what it shows. **Status:** done

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Inventory the remaining text decisions and their refusals, and choose the codes — **DONE** | 30m | — |
| t002 | Code not-found on every surface — **DONE** | 1h | t001 |
| t003 | Code the name-conflict, workspace-cap and last-admin refusals on every surface — **DONE** | 1h | t001 |
| t004 | Code the GitHub and shell unavailable refusals on every surface — **DONE** | 30m | t001 |
| t005 | The dashboard decides by code, and the protected retry reads the refusal's `name` — **DONE** | 1h30m | t002, t003, t004 |
| t006 | Render parity — **DONE** | 20m | t005 |
| t007 | Simplify — **DONE** | 15m | t006 |
| t008 | Test coverage — **DONE** | 45m | t007 |
| t009 | Closeout — **DONE** | 10m | t008 |

## Inventory (t001)

Each dashboard decision that still reads server copy, the refusal behind it, and the code it gets:

| Dashboard decision | bex-api refusal today | Code |
| --- | --- | --- |
| `isNotFoundError` (document head, `isEnvGroupNotFound`, `service-detail-loader`); the not-found part of `logReadAccessDenied` | `core.ErrNotFound`, `core.NotFound(resource)`: no code | `NOT_FOUND`, which `logReadAccessDenied` already expects |
| `use-rename-database`, `use-rename-key-value` (`"already exists"`) | `NewConflictError("CONFLICT", …)` | keep `CONFLICT`, which since w6/m49 means a name taken in scope (`isNameConflictError`); add `{name}` |
| `use-ssh-keys` (`"already registered"`) | `%w: SSH public key is already registered` (`ErrConflict`) | `SSH_KEY_EXISTS` |
| `use-create-database`, `use-create-key-value`, `use-create-service` (`"workspace is limited"`) | `core.quotaCapExceeded`: `ErrBadRequest`, no code | `WORKSPACE_RESOURCE_LIMIT` with `{limit}` |
| `use-remove-member`, `use-change-role` (`"last admin"`) | `members` and `store.ErrLastAdmin`: no code | `LAST_ADMIN` |
| `isGitHubUnavailable` (`"not configured"`, `"unavailable"`) | `core.ErrGitHubUnavailable`: no code | `GITHUB_UNAVAILABLE` |
| `use-shell-session` (`/not configured/`) | `core.ErrShellUnavailable`: no code | `SHELL_UNAVAILABLE` |
| agent sessions' `UNAVAILABLE` regex | already `AGENT_SESSION_NOT_CONFIGURED` | dashboard only |
| `protectedServiceName` parses the confirm phrase | `PROTECTED_ENVIRONMENT_CONFIRMATION_REQUIRED` already sends `name` | dashboard only |

Two code checks in `logReadAccessDenied` look for codes bex-api never sends: `UNAUTHENTICATED` and `NOT_FOUND`. `NOT_FOUND` becomes real here. The unauthenticated part also covers a 401 through `isUnauthenticatedError`, so t005 checks whether its regex still has a case to catch.

Out of scope: `isSandboxCapacityFailure` (`agent-sessions/lib/mapper.ts`) reads a session's stored `failureReason`, not a refusal. It needs a reason-code field, as w5/m129 added for Key Value, and is filed as w5/122.

## Definition of done

- Every refusal behind a dashboard decision that t001 lists carries one stable code: GraphQL `extensions.code`, the REST error body's `code`, and the MCP error text's `CODE: ` prefix. That covers a missing resource, the database, Key Value and SSH-key name conflicts, the workspace resource cap, the last admin, and the GitHub and shell unavailable refusals. Messages are unchanged, except that a coded refusal drops its `bad request: ` or `conflict: ` sentinel prefix.
- The dashboard decides each of them by code. No substring or regex of server copy remains in those decisions: `isNotFoundError` and its callers, `logReadAccessDenied`, both rename hooks, `use-ssh-keys`, the three create hooks' cap check, both member hooks, `isGitHubUnavailable`, `use-shell-session`, and the agent-session unavailable check.
- The protected-confirmation dialog reads the resource name from the refusal's `name` param, not by parsing the confirm phrase.
- Each code is pinned on every surface that makes it (`api/refusal_codes_test.go`, the SSH-key surface test, and the members unit and end-to-end tests). Dashboard tests decide by `codedGraphQLError`, each with a negative case showing the old wording alone no longer decides.

## Source + Goal linkage

- **Source:** promoted from inbox w5/111, found by w5/m128's t005 sweep (2026-10-06). The same sweep found the agent-session check during t001's look-ahead.
- **Goal linkage:** Render parity of error semantics (ADR006, ADR018). The dashboard is a client of bex-api's codes, not of its English copy. This finishes what w5/m125 and w5/m128 started.
- **Expected outcome:** bex-api can reword any of these messages, or localize the server's copy later, without breaking a dashboard decision. Each decision has a test that fails if it falls back to wording.
- **Why now:** m128 left the code plumbing in place, `core.CodedError` and `codedGraphQLError` among it, so each remaining refusal costs minutes rather than design. The not-found check also drives page titles and loaders on several resource pages.
- **Render parity:** included. The change touches REST, GraphQL and MCP error shapes.

## Result

Done 2026-10-07. Every dashboard decision t001 listed now reads a code:

| Refusal | Code |
| --- | --- |
| Missing resource | `NOT_FOUND`, or a feature's own `*_NOT_FOUND` |
| Taken name | the established `CONFLICT` |
| SSH key already registered | `SSH_KEY_EXISTS` |
| Workspace cap | `WORKSPACE_RESOURCE_LIMIT` |
| Last admin | `LAST_ADMIN` |
| GitHub unconfigured | `GITHUB_UNAVAILABLE` |
| Shell unconfigured | `SHELL_UNAVAILABLE` |
| Agent sessions unconfigured | `AGENT_SESSION_NOT_CONFIGURED` |

The protected dialog names the resource from the refusal's `name`. A transport 404 is no longer mistaken for a missing resource. Follow-up: w5/122, the agent-session sandbox-capacity reason.
