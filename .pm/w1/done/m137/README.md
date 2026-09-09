# w1 · m137 — Recover interrupted local agent-image updates

**Worker:** worker1 **Goal:** retries converge every local agent node to the intended image after interrupted imports. **Status:** done

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Resolve comparable local and node image identities — **DONE** | 45m | — |
| t002 | Reconcile node images after partial imports — **DONE** | 50m | t001 |
| t003 | Verify imports before workload rollout — **DONE** | 40m | t002 |
| t004 | Drill interrupted import and document recovery — **DONE** | 45m | t003 |
| t005 | Simplify — **DONE** | 20m | t004 |
| t006 | Test coverage — **DONE** | 30m | t004, t005 |
| t007 | Closeout — **DONE** | 10m | t006 |

## Definition of done

On disposable CAPD nodes, interrupt an update after only one node receives the new image. Rerun without editing source or setting AGENT_REBUILD: every node converges to the intended image identity. Already-correct images avoid reimport. Missing images are imported; inspection errors, import errors, and post-import identity mismatches stop before workload rollout and identify the image/node. Record live drill evidence and meaningful regression coverage. No successful rollout may conceal a stale node image.

## Source + Goal linkage

- **Source:** approved 2026-09-08 pm-brainstorm proposal 1; w1/done/081.md and scripts/dev-env.sh node-load loop (around lines 969–995) plus agent_build_images (around line 1124).
- **Gap:** September 2's fix reloads images changed during the current invocation. AGENT_RELOAD_IMAGES resets on the next invocation; a node retaining an old same-named tag can then be skipped. This is code-derived evidence, not a claimed live reproduction; t002 reproduces it and t004 supplies live proof.
- **Goal linkage:** ADR008 agent capabilities through a trustworthy local verification environment for ADR047 agent sessions.
- **Expected outcome:** developers can retry interrupted updates without unknowingly testing stale binaries.
- **Why now:** completes the recently shipped stale-image fix; w1 has capacity and the original fix's provenance. Does not duplicate w3/m79's remaining GitHub draft-PR proof.
- **Dependencies and coordination:** no unfinished implementation prerequisite; coordinate shared script and local cluster use with w3/m79. Use disposable nodes for fault injection. An unavailable live drill remains an explicit closeout blocker.
- **Sizing:** 180m implementation + 60m standing tasks, 7 tasks total.
- **Render parity omitted:** local development scripts and runbook only; no REST/GraphQL/MCP/UI contract change.
- **Scope:** recover image distribution after interruptions. Preserve current build-source freshness policy; no new registry, production rollout, or broad local-stack redesign.
