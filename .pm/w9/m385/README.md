# w9 · m385 — Agent secrets apply on their own: workspace-wide and per-repo scopes (Devin parity)

**Worker:** worker9 **Goal:** a workspace agent secret can be scoped to the whole workspace or to one repository and then reaches every matching session — API, dashboard, automation, `/bex` takeover, PR-feedback steer and child — without being named, as Devin's organization and repository secrets do; `explicit` keeps today's attach-by-name. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](t001.md) | ADR098: secret scopes, which session origins they reach, and the migration default | 45m | — |
| [t002](t002.md) | Store: persist each workspace agent secret's scope and repository | 60m | t001 |
| [t003](t003.md) | Dispatch: every session origin receives the secrets whose scope matches it | 90m | t002 |
| [t004](t004.md) | REST, GraphQL and MCP: read and set a secret's scope; the session view names what applied and why | 90m | t003 |
| [t005](t005.md) | Dashboard: scope picker on the Agent secrets card; applied secrets on the session page | 90m | t004 |
| [t006](t006.md) | Live check on production: unnamed sessions and an automation receive workspace and repo secrets; another repo's session does not | 45m | t005 |
| [t007](t007.md) | Render parity (Devin): every surface this milestone touched agrees, and matches Devin | 20m | t006 |
| [t008](t008.md) | Simplify | 15m | t007 |
| [t009](t009.md) | Test coverage | 45m | t007, t008 |
| [t010](t010.md) | Closeout | 10m | t009 |

## Definition of done

On production (QA user):

- A secret with scope `workspace` is in the environment of a new API session created without `secretNames`, of a steer on it, and of an automation run whose template names no secrets.
- A secret with scope `repo` for repository X reaches sessions on X and is absent from a session on another repository.
- A secret with scope `explicit` reaches only sessions that name it, as today.
- A session's view lists the applied secret names with their source (`inline`, `explicit`, `repo`, `workspace`) on REST, GraphQL, MCP and the dashboard; no surface returns a value, and every applied value is redacted from the transcript.
- ADR098 records the scopes, the origins they reach, the defaults and the open sign-off.

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w9` (2026-11-13), item 1, filed by `/pmx` 2026-11-13.
  - Devin: organization secrets "apply to the entire organization" and reach all future sessions; repository secrets are available "when it works in that repository" ([Secrets](https://docs.devin.ai/product-guides/secrets), checked 2026-11-13). Devin's v3 `SessionCreateRequest` also takes `secret_ids` (explicit attach).
  - bex: a workspace agent secret reaches a session only when named in `secretNames` (`docs/ADR098-agent-session-secrets.md:32`; `lego/backend/internal/agentsessions/secrets.go:261`). The `/bex` takeover creates its session with no secrets (`pr_takeover.go:210-217`); an automation passes only its template's `secretNames` (`automation_scheduler.go:232`). Skills are unaffected: dispatch delivers every workspace skill (ADR099 D5).
- **Goal linkage:** ADR008 AI-native pillar; Devin parity for cloud coding agents (ADR047).
- **Expected outcome:** `/bex fix the failing test` on a repository whose tests need credentials runs with them, and automations stop needing a hand-kept secret list.
- **Why now:** m344 and m348 shipped PR-feedback steers and the `/bex` takeover; both start or continue sessions that receive no secrets today, so the gap surfaces as soon as the user enables their GitHub App events.
- **Open sign-off (the brainstorm's recommendation, not a user decision):** new secrets default to `workspace` (Devin); existing secrets read as `explicit` so nothing stored under "only sessions that name it" widens without its owner acting. That is a divergence from Devin for pre-existing secrets and needs the user's sign-off; the alternative (every existing secret becomes `workspace`) is one default in t002. Under ADR008 parity-first the tasks implement the recommendation.
- **Render parity:** included (t007), against Devin, because the change touches REST, GraphQL, MCP and the dashboard.
