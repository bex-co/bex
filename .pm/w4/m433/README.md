# w4 · m433 — A restart keeps the running shutdown delay, port and pre-deploy command instead of shipping undeployed settings, as Render's Restart keeps the running configuration

**Worker:** worker4 **Goal:** a restart reruns the configuration the instance is running, including its shutdown delay, port and pre-deploy command; a rollback runs the target's; an undeployed save waits for the next deploy and stays reported until then. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Release records keep the shutdown delay, port and pre-deploy command each release ran with | 45m | — |
| t002 | Restart and rollback run the recorded shutdown delay, port and pre-deploy command | 45m | t001 |
| t003 | `undeployedChanges` keeps reporting the saved values through a restart | 40m | t002 |
| t004 | ADR004's residual and ADR018's Restart and Rollback rows | 15m | t002 |
| t005 | Live check on production: save a new port and shutdown delay, restart, then deploy | 30m | t003, t004 |
| t006 | Render parity | 20m | t005 |
| t007 | Simplify | 15m | t006 |
| t008 | Test coverage | 30m | t006 |
| t009 | Closeout | 10m | t007, t008 |

## Definition of done

1. On production in `bex-canary`, t005 records: after saving a new port and shutdown delay without deploying, a restart keeps the old values and the undeployed notice remains; the next deploy applies them; a rollback to the first deploy runs the old values while saved settings read the new ones. The service is deleted afterwards (by-id `GET` 404).
2. A release record written before this milestone still restarts, using the saved values, and the fallback is logged.
3. ADR004 no longer carries the w8/m135 residual for these fields, and ADR018's Restart and Rollback rows list them.
4. `cd lego/backend && go test ./...`, `make test` and `make lint` in `lego/operator/`, and dashboard `yarn lint` + `yarn test` pass.

## Source + Goal linkage

- **Source:** `/pm-brainstorm for one aspect of render parity` (the Deploys aspect), 2026-11-16, item 3; filed by `/pmx them all to w4` the same day.
- **Render reference:** render.com/docs/deploys § Restarting a service: a restart's new instance "always uses the exact same Git commit and configuration as the running instance at the time of the restart"; render.com/docs/rollbacks lists what a rollback restores and does not mention these three fields.
- **bex today:** ADR004 § Settings saves wait for a deploy (w8/m135), Residual: restart and rollback "consume a pending save. Their selected record supplies the start command, health path, environment and instance count. The remaining saved fields (shutdown delay, pre-deploy command, port) still come from the saved spec." `ReleaseRecordSpec` (`lego/types/v1alpha1/releasesnapshots.go`) has no field for them.
- **Decision state:** for rollback, selecting the target's recorded values extends ADR004's existing rule; Render's page is silent on these fields, so it is this proposal's recommendation under ADR008 Render parity first, not a user decision. Whether a Render restart runs the pre-deploy command is unverified; t002 records what Render's pages say.
- **Goal linkage:** ADR008 Render parity first; ADR018 § Deploys (Restart, Rollback); ADR004.
- **Expected outcome:** pressing Restart never puts a saved-but-undeployed port or shutdown delay live, so a port typo cannot take a service down through a restart.
- **Why now:** the residual is recorded and unowned, and it shares `targetReleaseConfig` with m432; landing them in sequence on one workstream avoids conflicts.
- **Render parity:** included: restart and rollback results and `undeployedChanges` read on REST, GraphQL, MCP and the dashboard change.
