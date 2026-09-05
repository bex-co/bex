# w5 · m76 — Dashboard "Update Source" card: swap a service's backing repo/branch/image from Settings

**Worker:** worker5 **Goal:** the last open GitHub-integration parity item (ADR026 §8 plan item 1) closes — a service's backing source (repo + branch, or container image) is viewable and editable from its dashboard Settings page, driving the already-shipped `PATCH /v1/services/{id}` fields, with save-without-deploy semantics **Status:** in progress — t001–t005 DONE locally; 2026-09-04 live audit found missing repository-access validation, now fixed locally; t006 awaits this fix’s ship + production verification

## Tasks (in order)

| id   | title                                                                                     | est | depends_on |
| ---- | ----------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Backend: verify + align PATCH source-swap semantics (repo↔image, no auto-deploy, ×3 surfaces) — **DONE** | 45m | —          |
| t002 | Dashboard: Source card on service Settings (display + Edit dialog + no-auto-deploy note) — **DONE** | 90m | t001       |
| t003 | Render parity — mirror Render's Update Source dialog semantics; record any divergence — **DONE** | 20m | t002       |
| t004 | Simplify — `/simplify` over the milestone's changed code — **DONE**                         | 30m | t003       |
| t005 | Test coverage — swap transitions, validation, dialog wiring — **DONE**                      | 45m | t003       |
| t006 | Closeout — verify DoD, sync status, move to done/                                          | 15m | t005       |

## Definition of done

On a live environment: a repo-backed service's Settings page shows its Source (`owner/repo` + branch); Edit opens a dialog offering (a) a different repo from the workspace's connected accounts (account-grouped picker), (b) a different branch, (c) switching to a container image — and saving PATCHes the service **without triggering a deploy** (the next deploy uses the new source, matching Render's Update Service API contract). An image-backed service can likewise be re-pointed or switched to a repo. Validation refuses a repo whose owner matches no workspace connection with a clear message (public repos allowed as public-git, per the create flow's rules). REST/GraphQL keep identical field semantics; suites green.

## Source + Goal linkage

- **Source:** [docs/ADR026-github-integration.md](../../../docs/ADR026-github-integration.md) §8 "Render-parity status & remaining plan" item 1 (2026-08-21), grounded in the live Render-dashboard walk: Render's Settings → Source **Edit** (May-2026 "Update Source" changelog) swaps repo/branch or repo↔image; bex already has the API half (`PATCH /v1/services/{id}` accepts `repo`/`branch`/`image` since w1/073 matched Render's PATCH field set) but no dashboard affordance.
- **Goal linkage:** Render parity (ADR018) on the service-configuration surface; completes the GitHub-integration parity plan — after this, ADR026 §8's "remaining" list is empty except the unscheduled grant-staleness polish.
- **Expected outcome:** a mis-pointed or migrated service is fixed from the UI in seconds (today it needs a raw API call); the ADR026 §8 plan item closes.
- **Why now:** it is the single remaining functional gap from the 2026-08-21 parity audit, small and well-bounded, and the m74 account-grouped repo picker it reuses just shipped.
- **Render parity task included:** yes — service-facing UI + the PATCH surface semantics must stay Render-consistent across REST/GraphQL.

## Constraints

- **No new backend verbs** — the PATCH fields exist; t001 only verifies/repairs semantics (e.g. repo⇄image transitions clearing the other source, `autoDeploy` untouched, no implicit deploy).
- Reuse `ServiceSourcePicker` (account-grouped, workspace-scoped per ADR078 §6) rather than a second repo picker.
- Type is immutable (Render: "You cannot change an existing service's type") — the dialog never offers a type change.

## Implementation evidence (2026-08-25)

- REST, GraphQL, and MCP share the atomic source patch. Repo/image transitions clear the other kind, preserve `autoDeploy`, open no deploy row, and mark the source generation pending; the operator pins the active artifact until a deploy generation consumes it.
- Source-save credential checks are read-only. A real deploy refreshes the new repo's workspace-owned clone token and materializes the selected registry credential, so saving cannot mutate credentials used by the active release.
- The non-static/non-cron Settings page has one Source card for repo and image services. Its Edit dialog reuses `ServiceSourcePicker`, including account-grouped connections, public Git, branch free text, image, and registry credential modes. The pending Source skeleton mirrors the ready card's compact two-column summary geometry.
- Simplify review applied reuse, efficiency, and clarity findings: one grouped picker query, lazy branch loading, no-op save detection, an object-shaped repo+branch mutation API, source-only legacy hooks isolated to cron/static variants, and an explicit untracked backend patch helper.
- Verification: dashboard typecheck/lint plus 2,671 tests; complete backend tests; operator `make test`; four-module `make lint`; all green. Official Render evidence was rechecked on 2026-08-25. Render's REST update endpoint saves without deploying, while its May 11 dashboard dialog deploys immediately and reconfirms runtime/build/start settings; ADR018 and ADR026 record bex's deliberate save-first dashboard divergence.


## Live audit and repair (2026-09-04)

- **Shipped baseline:** `fa7b99932` is an ancestor of production deployment `f6c4506e6f2c0029b843ec173b1f6904efce0f02`; [deploy run 33838301139](https://github.com/bex-co/bex/actions/runs/33838301139) completed successfully, including dashboard, backend, operator, controller, and rollout jobs.
- **Live scratch resource:** `qa-m76-source-20260904` (`srv-dadkhebr12dc73ba2grg`) in the QA user's `bex` workspace. The Settings Source dialog displayed the workspace's connected `bex-co` repositories, the three source modes, branch input, and save-without-deploy copy. Desktop and 390px mobile captures are in the ignored `.playwright-mcp/m76-*.png` artifacts.
- **Verified saves:** image → private connected repo `bex-co/bex-hello-go-live`, branch `main` → `qa-m76-pending` → `main`, and repo → `nginx:stable` all persisted through the UI. REST deploy-history reads before/after showed no new deploy; GraphQL source reads confirmed one source kind and `autoDeploy: false` throughout. REST also restored repo + branch without deploying.
- **Manual deploy evidence:** the dashboard's Manual Deploy → Deploy latest commit created `dep-dadkj5188i5c73999g2g`, resolving the selected private repo to `7ad20c989b805b440a88ce2d61b7c283bd44bf2c`. It progressed from queued to update-in-progress, but never reached a verified healthy endpoint during the audit (HTTP 503). This proves source resolution, **not** a successful end-to-end rollout or preservation of a healthy active artifact.
- **Missing requirement found:** submitting `https://github.com/m76-unconnected-owner/private-repo` succeeded even though the owner had no workspace connection and the repo was not public. The shared source setter had no repository-access check. This contradicts the validation DoD; the milestone remains open.
- **Local repair:** changed GitHub sources now validate through the existing owner-scoped grant check, then an anonymous repository lookup for public Git. Inaccessible private/missing repositories return a clear bad-request error; provider failures propagate. Validation runs before the pending marker, source-store write, or clone-secret removal. It writes no clone credentials. Other Git providers and installations with GitHub integration disabled retain the existing public-Git create behavior.
- **Regression coverage:** connected private, unconnected public, inaccessible/ungranted repos, wrong-workspace grants, provider failures, REST/GraphQL/MCP rejection without App/credential/deploy mutation, and keeping the rejected dashboard draft open. Simplify's three reviews completed; only import grouping needed cleanup.
- **Local verification:** complete backend `go test ./...` passed (optional integration services were not started locally); the final apps/GitHub packages passed again after adding the MCP rejection case. `make lint-backend` reports zero issues. Dashboard `yarn lint` (including typecheck) and all 394 test files / 2,899 tests passed, including the new rejected-draft test. Repository-wide Markdown formatting and `git diff --check` passed.
- **Cleanup:** scratch service deleted through `deleteService`; subsequent REST GET returned 404. Isolated browser contexts were closed and the audit login storage file removed. No existing service was edited.

### Remaining closeout gates

1. Ship the local access-validation fix and its tests, then verify its production rollout. The repository requires an explicit `$ship` invocation before commit/push.
2. Re-run the live rejected-private/missing and accepted-public cases after rollout; confirm the error is visible and the rejected save leaves source/deploy history unchanged.
3. Start with a healthy scratch service, repeat repo↔image saves, and prove the healthy active artifact stays in place until a manual deploy successfully activates the new source. The prior audit's 503 cannot satisfy this gate.
4. Complete the desktop/mobile pending-versus-ready geometry check and record the final evidence, then move t006 and the milestone to `done/`.
