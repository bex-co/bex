# w4 · m146 — Keep empty Blueprint environments idempotent

**Worker:** worker4 **Goal:** an unchanged Blueprint with an empty service environment produces no service change and no deployment. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Canonicalize empty merged Blueprint environments | 25m | — |
| t002 | Verify shared callers and preserve environment ownership | 35m | t001 |
| t003 | Render parity across validation, apply and dashboard plans | 20m | t002 |
| t004 | Simplify | 10m | t003 |
| t005 | Test coverage | 30m | t003 |
| t006 | Closeout | 10m | t004, t005 |

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
