# w4 · m197 — Restore a static rollback's retained publication

**Worker:** worker4 **Goal:** restoring an earlier static publication works after a bad saved publish-directory change **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Bind a static target to its retained published revision | 50m | — |
| t002 | Publish a rollback from retained content | 60m | t001 |
| t003 | Audit shared rollback callers and publication modes | 60m | t002 |
| t004 | Render parity | 20m | t003 |
| t005 | Simplify | 20m | t004 |
| t006 | Test coverage | 45m | t004 |
| t007 | Closeout | 10m | t005, t006 |

## Definition of done

- On an owned built static site, publish v1 from `dist`, publish v2, then save a nonexistent publish directory and wait for its specific `update_failed` result while v2 remains publicly served.
- From a fresh deploy page, roll back to the previously Live v1 target without repairing that saved directory. The rollback reaches Live and the public body is v1. It uses the target's retained publication rather than extracting the old image through the current directory or executing today's build command.
- After rollback, the saved v2 build command and nonexistent publish directory remain unchanged, and the existing dashboard auto-deploy policy is preserved. The rollback row names the exact target and has the correct finished status.
- Publication-copy failure preserves the prior serving content and gives a specific failed verdict. A missing retained artifact is refused with a named reason before dispatch; an empty copy cannot be reported as successful.
- The separate t003 audit records all three API adapters, the shared static publisher, built/direct modes, legacy targets and dashboard entry points. Mark live-unreplayed branches explicitly; the source finding proves only built static rollback.
- Delete all owned fixtures and revoke only the QA session; verify the original inventory and signed-out state.

## Source + Goal linkage

- **Source:** functional $qa-find-bugs w4, 2026-10-10 pass a79, muse.env login helper. [Finding with durable API receipts](finding.md). Severity major.
- **Goal linkage:** dependable deployment recovery in [ADR004](../../../docs/ADR004-app-deployment.md), static hosting in [ADR029](../../../docs/ADR029-static-sites.md), and hosting parity in [ADR018](../../../docs/ADR018-render-parity.md).
- **Expected outcome:** a user can restore an earlier published static version after breaking the saved output directory.
- **Why now:** the same exact v1 image fails rollback with today's bad directory and succeeds immediately after restoring `dist`. This is a publication gap left unverified by w4/m141, rather than a recurrence of its silent-dispatch failure.
- **Scope/size:** approximately 4h25 across retained-publication identity, publisher support, compatibility audit and meaningful verification. This does not implement a product fix during the QA filing.
- **Render parity:** included because rollback is exposed through REST, GraphQL, MCP and the dashboard; retained-artifact semantics and saved settings must agree.
- **Cleanup:** owned site deleted, REST 404, all six baseline ID families restored, own session revoked, old whoami 401, cookies 0 and browser about:blank. Local ledger cleanupVerified true.
