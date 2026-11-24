# w32 · m608 — Each deploy records its own Git repo and branch so switching a service's `--repo`, `--branch`, or `--image` never rewrites historical build logs or log IDs

**Worker:** worker32 **Goal:** every Git-backed deploy row records the `repo` and `branch` it ran from, and `buildProgressLines` narrates from that per-deploy record rather than the service's current `App.Spec`, so switching a service's `--repo`, `--branch`, or `--image` leaves every historical deploy's `==> Build queued`, `==> Building from <repo>@<sha|branch>`, and `==> Deploying...` lines — and their deterministic `logID`s — immutable across CLI, REST, subscribe stream, GraphQL, MCP, and the dashboard. **Status:** blocked (pin must carry `4917e03d5e`; t001–t003, t006, t007 done)

## Tasks (in order)

| id   | title                                                                                               | est | depends_on |
| ---- | --------------------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | [Record each deploy's Git repo and branch on its `deploys` row](done/t001.md) — **DONE**            | 1h  | —          |
| t002 | [Narrate deploy progress lines from per-deploy `repo` and `branch`](done/t002.md) — **DONE**        | 1h  | t001       |
| t003 | [Document per-deploy source immutability in the progress-line contract](done/t003.md) — **DONE**    | 20m | t002       |
| t004 | [Ship and verify on production](t004.md)                                                            | 45m | t002, t003 |
| t005 | [Render parity](t005.md)                                                                            | 30m | t004       |
| t006 | [Simplify](done/t006.md) — **DONE**                                                                 | 15m | t005       |
| t007 | [Test coverage](done/t007.md) — **DONE**                                                            | 30m | t005       |
| t008 | [Closeout](t008.md)                                                                                 | 10m | t006, t007 |

## Definition of done

- On production, once the pin carries this milestone's commits, the released `bex` passes this walk in `bex-canary`:
  - Create a disposable free native Node web service (`examples/hello-node` from `https://github.com/bex-co/bex`, `--plan free`), wait for its initial deploy to reach `live`, and record its `==> Building from https://github.com/bex-co/bex@<sha>` line and `id` from `bex logs -r <id> --type build -o json`.
  - Switch the service's `--repo` to another repo (e.g. `https://github.com/expressjs/serve-static`) and re-read `bex logs -r <id> --type build --text "Building from" -o json`: the initial deploy still reports `==> Building from https://github.com/bex-co/bex@<sha>` with the identical `id`.
  - Switch the service to a prebuilt image (`--image docker.io/library/nginx:1.27-alpine`) and re-read `bex logs -r <id> --type build -o json`: the initial deploy still reports `==> Build queued`, `==> Building from https://github.com/bex-co/bex@<sha>`, and `==> Deploying...` with identical `id`s (never rewritten to `==> Deploy queued` / `==> Rolling out the current release`).
  - Verify REST `GET /v1/logs`, GraphQL `logs`, MCP `list_logs`, and the dashboard deploy log query agree on the historical deploy's lines and IDs, and `GET /v1/services/{id}/deploys` exposes no unrequested public fields.
  - Delete the fixture by ID and confirm `404`.
- Store and real-Postgres tests cover:
  - Recording `repo` and `branch` on every Git-backed deploy insert/promotion path (`CreateApp`, `CreateDeploy`, `CreateRestartDeploy`, `EnsureCheckedDeploy`, `promoteDeferredDeploy`, and leaving non-build image/override/rollback rows accurate).
  - Historical deploy rows keep their original `repo` and `branch` after `SetAppSource` changes repo, changes branch, or switches to a prebuilt image.
  - Pre-migration rows (`repo = ''`, `branch = ''`) remain readable and fall back to the App's current source without breaking existing deploys.
- Logs unit and API tests cover:
  - Repo A → Repo B switch leaves historical deploy's `==> Building from <repo A>@<sha>` and `id` unchanged.
  - Branch A → Branch B switch leaves an unresolved-commit historical deploy's `==> Building from <repo A>@<branch A>` and `id` unchanged.
  - Git → prebuilt image switch leaves historical Git build deploy's `==> Build queued`, `==> Building from <repo A>@<sha>`, `==> Deploying...`, and `id`s unchanged.
  - Prebuilt image → Git switch leaves historical image deploy's `==> Deploy queued` / `==> Deploying image <image>` and `id`s unchanged.
  - Pre-migration fallback (`d.Repo == ""`) falls back to `pc.repo` / `pc.branch`.
  - Live tail (`progressFollower`) and history reads agree across REST, SSE/WebSocket subscribe, GraphQL, and MCP.
- `docs/render-artifacts/live-deploy-following.md` § Platform progress-line contract records that `repo` and `branch` are captured on each deploy row so historical build progress lines and log IDs are immutable across source switches.

## Source + Goal linkage

- **Source:** inbox note [w32/106](../../done/106.md), filed 2026-11-23 from routine `cli-qa` sweep `8b71` (released `bex` v0.9.0 in `bex-canary`) and promoted 2026-11-24 by `/loopx w32`. Fixing the bug cleanly spans a new `deploys` SQL migration, all store deploy-creation/promotion paths with real-Postgres tests, `DeployProgress` + `buildProgressLines` / `rolloutLinesWith` narration across all log surfaces, contract documentation, and a post-deploy production walk (~4h30m across 8 tasks).
- **Goal linkage:** Render parity ([ADR008 § Render parity first](../../../../docs/ADR008-vision.md#render-parity-first)) and auditability of deploy logs. Historical deploy logs on Render are an immutable record of what each deploy actually built; rewriting past logs to name a newly attached repository falsifies commit provenance and churns log IDs.
- **Expected outcome:** switching a service's `--repo`, `--branch`, or `--image` affects only subsequent deploys; every earlier deploy keeps its exact `==> Build queued` / `==> Building from …` / `==> Deploying...` lines and deterministic `logID`s on every log surface.
- **Why now:** `w32/m560`, `w32/m567`, `w32/m575`, `w32/m576`, and `w32/m591` already made per-deploy rollout, live URL, and exit narration immutable on the deploy row (like `detected_http_port` before them). Recording `repo` and `branch` closes the remaining App-spec leak in `buildProgressLines` found during live CLI QA.
- **Render parity task included:** build progress lines and deploy DTOs are tenant-facing across CLI, REST, subscribe stream, GraphQL, MCP, and the dashboard deploy log pane.
