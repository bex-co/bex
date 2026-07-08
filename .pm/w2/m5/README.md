# w2 · m5 — Deploy history + trigger (`list_deploys` · `get_deploy` · `POST /deploys`)

**Worker:** worker2 **Goal:** Give agents Render's deploy poll-loop: every rollout is recorded as a deploy object (`dep-…`, status `build_in_progress → update_in_progress → live/failed`, per docs/deployment.md's health gating), listable over REST/GraphQL/MCP and triggerable via `POST /v1/services/{id}/deploys`. **Status:** todo

## Tasks (in order)

| id   | title                                                                                                    | est | depends_on   |
| ---- | -------------------------------------------------------------------------------------------------------- | --- | ------------ |
| t001 | Deploys table in `lego/backend/internal/store` + recording: transitions stamped from App revision/phase   | 30m | w1/m2        |
| t002 | REST: `GET /v1/services/{id}/deploys` + `GET …/deploys/{deployId}`, Render envelope/shapes                 | 25m | t001         |
| t003 | `POST /v1/services/{id}/deploys` (trigger): image re-pull/restart now; build-from-git activates with w1/m5 | 25m | t001         |
| t004 | MCP `list_deploys`/`get_deploy` + GraphQL dashboard `deploys` query over the same verb                     | 25m | t002         |
| t005 | Acceptance: create → deploy recorded → trigger → second deploy reaches `live`; failed image reads `failed` | 25m | t003, t004   |
| t006 | Simplify — `/simplify` over the code this milestone changed                                                | 20m | t005         |
| t007 | Test coverage — meaningful tests for the behavior this milestone shipped                                   | 30m | t005         |

## Definition of done

`list_deploys` returns a truthful history whose latest entry's status matches the running reality (health-gated `live`, or `failed`); `get_deploy` fetches one by `dep-…` id; triggering a deploy produces a new record an agent can poll to `live` — the exact loop Render-trained agents run. All three surfaces read through one verb; requires the control-plane store (`BEX_CP_DB_URI`), returning 503 without it (omitted, not faked).

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w2` 2026-07-08; w2/m1's own deferral list ("no `list_deploys`/`get_deploy` — add them when `Core` grows those verbs, keeping Render's names"); Render OpenAPI deploys endpoints + `render-oss/render-mcp-server`; docs/deployment.md (revisions, health gating).
- **Goal linkage:** pillars 3/4 — deploy-from-chat is incomplete if the agent can't watch the deploy converge.
- **Expected outcome:** agents (and later the dashboard's Events/Deploys UI) can enumerate and poll deploys instead of inferring state from `phase` alone.
- **Why now:** the storage it needs (control-plane store, w1/m2) just shipped with only live acceptance (t007) pending, so this is newly unblocked; w2/m2's acceptance will want this observability half.
