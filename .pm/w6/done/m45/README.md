# w6 · m45 — Live QA hunt: native build PATH loss, invisible creation-time env vars, stale post-cancel status

**Worker:** worker6 **Goal:** a native Go/Rust web service actually builds and deploys; an env var set at service-creation time is visible, editable, and exportable through the same Environment tab/API that vars added later already use; and the service header's status pill matches the deploy's terminal state immediately after Cancel/Rollback, not after a 30s poll or a reload. **Status:** done 2026-08-22 — all 8 tasks complete; every suite green locally (operator `make test`, backend `go test ./...`, `make lint` 0 issues across 4 modules, dashboard lint + 2521 tests), and every fix proven red on the pre-fix code. **Closed unshipped:** nothing here is committed, so the three DoD lines that say *live* are owed by whoever runs `/ship` — see "What the shipper still owes" below. t001 is the exception: it was verified against the real digest-pinned toolchain images with Docker, which is why the Go/Rust rows below are ticked.

## Tasks (in order)

| id   | title                                                                                                                      | est | depends_on               |
| ---- | --------------------------------------------------------------------------------------------------------------------------- | --- | ------------------------ |
| t001 | Fix native build/start command losing PATH under the login-shell wrapper (breaks Go, likely Rust) — **DONE**                          | 45m | —                         
| t002 | Seed creation-time literal env vars into the secrets store so the Environment tab/API see them — **DONE**                              | 45m | —                         
| t003 | Refetch the Server query after deploy Cancel/Rollback so the header status pill isn't stale — **DONE**                                 | 30m | —                         
| t004 | Dashboard polish: dedupe live-tail reconnect log lines, runtime auto-detect on Root Directory change, deploy commit/date spacing — **DONE** | 30m | —                     
| t005 | Render parity across REST/GraphQL/MCP/UI — **DONE**                                                                                     | 30m | [t001, t002, t003, t004] 
| t006 | Simplify the touched code — **DONE**                                                                                                    | 30m | t005                     
| t007 | Test coverage for the fixed behaviors — **DONE**                                                                                        | 45m | t005                     
| t008 | Closeout — **DONE**                                                                                                                      | 10m | t007                     |

## Definition of done

- [x] Deploying a native Go OR Rust web service from a public repo builds and serves — **not** just via the generated Dockerfile text: the operator's own generated Dockerfile was built with real Docker/BuildKit against the digest-pinned `golang:1.24-bookworm` and `rust:1-bookworm` images, and both containers were started by the generated `CMD` and curled. Pre-fix, both died with `command not found` / exit 127 — the production build log exactly. Rust was only *reasoned* broken when this was filed; now confirmed. See t001 § Verification.
- [x] An env var set at creation time is immediately visible in `envVars(serviceId)` with no follow-up Edit+Save — and on REST and MCP too, not just the GraphQL path the hunt probed.
- [x] The header's status pill matches the deploy's terminal state immediately after Cancel/Rollback on all 3 surfaces — the fix is in `DeployActions` itself, the one component all three mount.
- [x] Regression tests exist for all of the above and fail on the pre-fix code — the exact red output for each is tabulated in t007.

## What the shipper still owes (live, on `dashboard.bex.co`)

None of this is committed, so the production re-probe is still outstanding:

1. Deploy a native **Go** and a native **Rust** service from a public repo; both reach Live and their `.onbex.co` URLs serve. (`examples/hello-rust` was added for this — std-only, no crates.io fetch.)
2. Create a web service with an env var in the New Web Service form; `envVars(serviceId)` and the Environment tab show it immediately, and reveal/edit/export/delete all work on it.
3. Cancel an in-progress deploy; the header pill leaves "Building" without a reload, on the deploys list, deploy detail, and events tab.
4. Re-verify a **Node/Python** native deploy still builds (the runtimes that accidentally survived the bug).

## Source + Goal linkage

- **Source:** live QA hunt of `dashboard.bex.co` hosting features, 2026-08-22 (via `/qa-find-bugs`, run from a `/loop 10m` session). Signed in as workspace "bex". Evidence under `.playwright-mcp/` (gitignored): `qa-services-1-go-build-fail.png`, `qa-deploys-1-text-concat.png`, `qa-env-1-missing-var.png`.
- **Goal linkage:** core hosting-journey correctness (deploy a service, manage its config, trust its status) — `docs/ADR004-app-deployment.md`, `docs/ADR006-bex-api.md`, `docs/ADR018-render-parity.md`.
- **Expected outcome:** a customer deploying a Go or Rust service from a public repo gets a working service instead of a build that fails with a confusing toolchain error; a customer who sets an env var at creation time can see/edit/export it like any other; canceling or rolling back a deploy never leaves the header lying about the service's state.
- **Why now:** found in a real live hunt of production; all three primary findings have clean, isolated, file:line-pinned root causes and t001/t002 are correctness bugs on core hosting journeys (create a service, manage its env), not polish. Render parity closing task **included** — t001 touches the build pipeline surface exposed identically across native runtimes, and t002/t003 touch REST/GraphQL/MCP/UI env-var and deploy-action surfaces directly.

## Investigated and rejected (not filed)

- **Free-tier service manually scaling past 1 instance.** This hunt observed scaling a Free-instance-type test service to 4 (of a 100 platform ceiling) real running replicas with no plan-based rejection. Checked before filing: `docs/ADR018-render-parity.md` line 66 documents this as a **deliberate, user-confirmed divergence** from Render (`w1/m81`, 2026-08-19 — "bex has no per-plan replica cap... this is a plan-policy difference, not a UI-validation gap"). Not a bug; no task filed. The test service was scaled back to 1 instance during this hunt's cleanup.
