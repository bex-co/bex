# w4 · m201 — Restart selects the revision already serving

**Worker:** worker4 **Goal:** Restart preserves the actually serving artifact and configuration when deploy history is still catching up. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Audit current-release selection and shared dispatch callers | 45m | — |
| t002 | Select the serving release within Restart dispatch and retries | 60m | t001 |
| t003 | Replay both cutover windows and settled controls | 45m | t002 |
| t004 | Render parity and surface verification | 25m | t003 |
| t005 | Simplify | 15m | t004 |
| t006 | Test coverage | 45m | t004, t005 |
| t007 | Closeout | 10m | t006 |

## Definition of done

Repeat the captured production dashboard journey on one owned Free prebuilt Python web service, same image/working command and synthetic V1/V2 environment marker. Timestamp complete requests/responses and public JSON reads; delete the service and revoke only the owned session afterwards.

- Deploy V2 successfully, then roll back to V1. After the public URL serves V1 and the service reports Running/current revision while the rollback history row is still In Progress, confirm Manual Deploy → Restart. The new Restart becomes Live and still serves **V1**, leaving saved V2 pending. It must not restore the older SQL-Live V2 configuration.
- Let a rollback V1 reach API Live, then Restart it. The Restart becomes Live, continues serving **V1**, and retains saved V2 pending: the settled positive control remains correct.
- From a fresh Deploys page, deploy saved V2. As soon as the public URL serves V2 and the service reports Running/current revision while that new row is still In Progress, confirm Restart. The new Restart becomes Live and still serves **V2**; it must not revert to the old SQL-Live V1 configuration or make saved V2 pending.
- A subsequent normal deploy still uses saved V2 and reaches Live with public 200/V2. No saved environment/command is rewritten by Restart. Historical intermediate rows remain honestly reported rather than being fabricated as Live to hide the selection error.

The exact window may require deterministic local status/history observation ordering plus a bounded owned production replay; a settled-only pass does not satisfy the two failing clauses. Other families, metadata edge cases and REST/MCP/CLI mutation paths are explicit verification work in t001/t004/t006, not observations claimed by this DoD.

## Source + Goal linkage

- **Source:** user-requested continuous functional qa-find-bugs using muse.env, explicitly w4, 2026-10-11 pass a209. [Complete evidence and researched fix](finding.md).
- **Goal linkage:** ADR008 dependable Render-compatible hosting, ADR004 selected-runtime configuration and snapshots, ADR018 Restart semantics, ADR006 shared API.
- **Expected outcome:** a user restarts what is already running even before its history row becomes Live; ordinary deploy and explicit rollback preserve their own existing behavior.
- **Why now:** normal dashboard Restart silently substituted configuration in two opposite-direction cutover windows; the settled control worked. Same-image configuration-only releases reproduce the failure without a build or artifact-change assumption.
- **Sizing:** 150m implementation/replay; 245m including standing closing tasks; 7 tasks. This is shared backend selection/serialization work with behavioral concurrency checks, not a sub-hour note.
- **Scope:** filing only. Localize the correction to Restart; preserve explicit Rollback target selection, saved state, current plan/domain policy and separate static/datastore behavior. t001 owns the counted dispatch blast radius and modern/legacy edge policy.
- **Precedent:** residual violation of completed w5/m107's Restart preservation guarantee. Its entire original DoD is dispositioned in finding.md. Existing open m105/m114/m159/m180/m184 have distinct stated scope and are not duplicated or closed.
- **Standing closing tasks:** Render parity, Simplify, Test coverage and Closeout apply because the common REST/GraphQL/MCP Restart verb and dashboard behavior are tenant-facing.
- **Cleanup:** owned service deleted; service/public 404, six full inventories exactly match baseline; owned session revoked/API 401, blank browser/zero cookies. Local complete ledger and public sampler paths are verified in finding.md. No foreign resource changed.
