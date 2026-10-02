# w4 · m143 — Initialize service-page environment-group scope on the first controlled opening

**Worker:** worker4 **Goal:** an environment-scoped service creates a group in its own environment on the first dialog opening after a cold page load. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Initialize scope and selection together for a controlled open | 40m | — |
| t002 | Verify both dialog consumers and service-route aliases | 30m | t001 |
| t003 | Render parity | 20m | t002 |
| t004 | Simplify | 15m | t003 |
| t005 | Test coverage | 30m | t003, t004 |
| t006 | Closeout | 10m | t005 |

## Definition of done

- Hard-load an environment-scoped QA service's Environment tab, then use the top **Create group** action. The first opening shows the service's actual environment and its checked, visible service; creating a named group succeeds in that scope. Repeat through the panel's **Create group** action after a fresh load. No cancel/reopen workaround is required.
- Cancel/reopen preserves correct defaults; the same first-open journey on a workspace-scoped QA service still creates a workspace group linked to that service.
- From `/env-groups`, Workspace lists the workspace-scoped control and excludes the environment-scoped control; selecting the QA environment lists its service and allows successful creation. These existing working branches remain usable.
- A deliberate mismatched GraphQL create still returns `ENV_GROUP_SERVICE_ENVIRONMENT_MISMATCH` without creating a group; equivalent REST POST still returns 409. Delete this drill's groups/services/project/environment and revoke its session.

These clicks and API probes were run in this hunt. MCP and other service types remain explicit verification work in t002/t003, not observed results.

## Source + Goal linkage

- **Source:** live `$qa-find-bugs` 2026-10-02, `muse.env` account, sweep r1; [research and full API evidence](finding.md). Verified local artifacts: `.playwright-mcp/qa-env-r1-1.png`, `.playwright-mcp/qa-env-r1-1.json` (gitignored).
- **Goal linkage:** ADR008 hosting usability, [ADR032](../../../docs/ADR032-environments.md) scope semantics and [ADR013](../../../docs/ADR013-secrets.md) shared configuration.
- **Expected outcome:** the first attempt to share configuration succeeds with a compatible scope and visible selection, without silently retaining a hidden incompatible service ID.
- **Why now:** this is the missed cold-load acceptance case from [w4/m111](../done/m111/README.md), whose live re-probe was deferred. The same backend correctly rejects the malformed first request and accepts the post-cancel request; weakening backend validation would hide the UI bug.
- **Render parity:** included because the shared create form builds GraphQL mutation inputs. Render does have environments; its workspace-group linking rule differs from bex's exact-scope rule. t003 must correct the old m111 parity assumption and record the divergence without broadening this fix.
