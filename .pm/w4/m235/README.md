# w4 · m235 — A new service's URL says its first deploy is in progress, not `404 page not found`

**Worker:** worker4 **Goal:** from creation until its first release serves, a public web service or static site answers bex's own "first deploy in progress" response, never Traefik's or the static-server's raw not-found text. **Status:** todo — t001–t006 done 2026-10-19 (loopx w4); live on pin `b4726243e74f` the first-deploy 503 answers from ~3 s after create, but a ~2.5 s raw-404 window remains right after the 201 (QA run 8a59, 2026-10-27) — t008 open, then t007 closeout

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Capture Render's pre-first-deploy response and fix the response contract — **DONE** | 20m | — |
| [t002](done/t002.md) | Web services: route a never-served public App to the activator from its first reconcile, and answer the first-deploy response — **DONE** | 60m | w4/m235/t001 |
| [t003](done/t003.md) | Static sites: route at creation and answer the first-publish response — **DONE** | 40m | w4/m235/t001 |
| [t004](done/t004.md) | Render parity — **DONE** | 15m | w4/m235/t002, w4/m235/t003 |
| [t005](done/t005.md) | Simplify — **DONE** | 10m | w4/m235/t004 |
| [t006](done/t006.md) | Test coverage — **DONE** | 40m | w4/m235/t004 |
| [t008](t008.md) | Close the first seconds after create, when the host still answers Traefik's raw 404 | 45m | w4/m235/t006 |
| [t007](t007.md) | Closeout | 20m | w4/m235/t005, w4/m235/t006, w4/m235/t008 |

Seven tasks, 205m (3h25m). One **minor** finding. This milestone schedules the work only.

## Definition of done

Replay [finding.md](finding.md) after the operator deploys. Create each fixture with `POST /v1/services`, then poll its host every 3 s from the 201 until the first `200`, logging every distinct status, `content-type`, `Retry-After` and body:

- **Docker web service** (Free, `https://github.com/bex-co/bex`, root `examples/docker-build-env`, env `GREETING`, secret file `.env`):
  - Every pre-200 answer is bex's first-deploy response: `503`, a `Retry-After`, and the t001 wording, content-negotiated (`Accept: text/html` gets a page, `*/*` gets text).
  - `404 page not found` and a bare `service unavailable` never appear.
  - The first `200` arrives no later than today's (about 1m40s).
- **Static site** (`examples/static-site`, `publishPath: .`): the same, with `no static site for host` never appearing before the first `200`.
- **Controls:**
  - A first deploy that fails (bad Dockerfile path) still ends on w1/m179's `503 service unavailable` without `Retry-After`.
  - Suspend still answers "This service is suspended."
  - A private service gets no public route.
- **Checks pass.** Operator `make test`, including the activator and static-server packages.
- **Cleanup.** No fixture remains, and only that run's session is revoked.

## Source + Goal linkage

- **Source:** live functional `/qa-find-bugs w4`, pass f26a22, 2026-10-19 UTC, `muse.env`, workspace `bex-canary`. See [finding.md](finding.md) (the polling tables are the probe record). Production was pinned to `db7e50ced338`.
- **Goal linkage:**
  - [ADR004](../../../docs/ADR004-app-deployment.md) (deploy lifecycle and routing) and [ADR029](../../../docs/ADR029-static-sites.md) (static serving).
  - [ADR007](../../../docs/ADR007-restart-suspend-and-resume.md): the activator's not-serving responses.
  - [ADR008](../../../docs/ADR008-vision.md): truthful product state. w2/m98, w6/m94 and w1/m179 established "never raw infrastructure output" for the other not-serving states.
- **Expected outcome:** someone who clicks a new service's URL, which the dashboard shows from creation, learns that the first deploy is still running and when to retry. They are not shown a not-found page that reads as their own app's 404.
- **Why now:**
  - This is the first public response every new service gives.
  - The pieces exist already: the activator alias, the content-negotiated responder and the static Ingress helper.
  - This finishes the not-serving family rather than adding a mechanism.
- **Render parity included:** the public response of a tenant host changes. t001 captures Render's actual pre-first-deploy answer and records parity or a deliberate divergence in ADR018. No REST, GraphQL, MCP or dashboard field changes.
- **Unverified:**
  - Paid web services (code-read: endpoint-less Service, so presumably Traefik's `503 no available server`).
  - Image-backed services' short window.
  - Custom domains attached before the first deploy.
