# w4 · m146 — Keep empty Blueprint environments idempotent

**Worker:** worker4 **Goal:** an unchanged Blueprint with an empty service environment produces no service change and no deployment. **Status:** done 2026-10-02 — all tasks complete; hosted DoD replay passed

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Canonicalize empty merged Blueprint environments — **DONE** | 25m | — |
| t002 | Verify shared callers and preserve environment ownership — **DONE** | 35m | t001 |
| t003 | Render parity across validation, apply and dashboard plans — **DONE** | 20m | t002 |
| t004 | Simplify — **DONE** | 10m | t003 |
| t005 | Test coverage — **DONE** | 30m | t003 |
| t006 | Closeout — **DONE** | 10m | t004, t005 |

## Definition of done

Repeat the exact probes in [the finding](finding.md) using new disposable names.

- Create the free static fixture with `envVars: []`, wait for live/HTTP 200, then validate its unchanged manifest over GraphQL, REST multipart and MCP. Each reports the existing service as `noop` with no changed-field paths.
- Apply that unchanged manifest twice, with a fresh service-page load between attempts. The service ID, deploy count, serving revision, App generation and restartedAt remain unchanged.
- Omit just envVars and repeat the control: plan/apply remain no-ops. A plan-only change of staticPublishPath to dist still reports update with exactly that changed path; a new QA name still reports create.
- Re-plan the observed web + fromGroup-only fixture unchanged. Its service action is noop with no changed paths; its existing env-group action keeps the deliberate conservative update.
- Delete the fixtures and revoke the QA session. Record the deployed version and exact live results before closeout.

Live sibling/resource-reference cases not exercised in the filing remain explicit verification work in t002, not claims of observed failures.

## Source + Goal linkage

- **Source:** `qa-find-bugs` 2026-10-02 sweep 6, user-directed w4 placement and `muse.env` login. [Finding](finding.md) contains complete replayable requests/responses, source trace, caller counts and prior-DoD disposition. Local screenshot: `.playwright-mcp/qa-r6-static-unchanged-rev3.png`.
- **Goal linkage:** ADR008 reliable hosting and agent operations; ADR049 Blueprint plan/apply semantics; ADR006 shared API business logic. Continues w1/m24's unchanged-apply guarantee and the empty-list normalization work in w4/m124.
- **Expected outcome:** identical infrastructure declarations leave workloads and deployment history untouched, while genuine environment updates still deploy.
- **Why now:** two identical production applies republished an unchanged site (rev-1 → rev-2 → rev-3); this is a repeatable deployment side effect, beyond plan copy. Existing canonicalization missed the environment merge helper. No open milestone covers it and the fix has not landed.
- **Sizing:** 130m across normalization, shared ownership/caller checks and serialized apply regressions, plus the standing closing tasks. The code edit is small; preserving the shared apply contracts is the substantive work.
- **Render parity included:** tenant-visible behavior changes through REST/GraphQL/MCP and both dashboard Blueprint plan consumers. Compare documented affected-resource and environment-preservation semantics; do not imply a Render account probe was run.

## Dedupe and limits

Read the finding's complete w1/m24, w6/m125, w4/m124 and w4/m138 disposition before implementing. This is an uncovered empty-env branch of the idempotence guarantee; there is no evidence the old domain/IP-allowlist fixes regressed. Keep env-group conservative plans, omission preservation and real changes intact. Git-based create/sync, other service types and reference forms are reasoned shared callers and require verification. The filing ships no product code.

## Blocked closeout — 2026-10-02

Implementation, full backend tests, backend lint and simplify review passed. The release pipeline must deploy the backend fix; QA must execute all live README controls and fixture/session cleanup. See [verification](verification.md) for exact local evidence and unverified surfaces.

## Supplemental checks — 2026-10-02

[Additional direct-apply, connected Git/auto-sync and actual group/seed handler coverage](supplemental-verification.md) extends the shipped tests. Hosted parity and closeout remain open.

## Live acceptance — 2026-10-02

Production carried fixes `683012552` and `7cb1dc64e` (ancestors of deployed `18958459c`). Workspace `tea-d98210cbbpdc73dcrkvg`; times UTC 2026-10-03 06:50–06:54. Manifests are the finding's, renamed to `qa-20261002-w4x-*`.

- **Create:** `POST /v1/blueprints/deploy` with the free static `envVars: []` manifest created `qa-20261002-w4x-static` (`srv-db0ab8qtm2ss7389qo6g`); `dep-db0ab8qtm2ss7389qo70` live 06:50:36; `https://qa-20261002-w4x-static.onbex.co/` HTTP 200; App generation 1, activeRevision rev-1, no restartedAt.
- **Validate unchanged (`envVars: []`):** GraphQL `validateBlueprint`, REST multipart `POST /v1/blueprints/validate` (file `render.yaml`) and MCP `validate_bex_yml` each return the service as `noop` with no changed fields (GraphQL `[]`; REST/MCP omit the field).
- **Apply unchanged twice** (dashboard service page freshly loaded between): same service ID, deploy count stays 1, revision rev-1, App generation 1, restartedAt absent after each apply.
- **Omitted envVars:** plan `noop` on all three surfaces; apply leaves the same ID, one deploy, rev-1, generation 1.
- **Controls:** `staticPublishPath: dist` → `update` with exactly `staticPublishPath` (all three surfaces); new name `qa-20261002-w4x-static-new` → `create`.
- **fromGroup web:** for `qa-20261002-w4x-group` + `qa-20261002-w4x-groupweb` (created by the w4/m148 replay), an unchanged re-plan returns the service `noop` with no changed paths and the group's deliberate conservative `update` (`envVars`, `name`) on all three surfaces.
- **Not exercised live:** the dashboard New Blueprint/Sync plan summaries (need a Git-hosted manifest with a `qa-` name); other reference forms and connected Git sync stay as t002/supplemental verification.
- **Cleanup:** static fixture deleted; REST GET → 404; no `w4x-static` App CR (read-only kubectl). The group/web fixtures are deleted with w4/m148's cleanup. QA session revoked at the end of the run.
