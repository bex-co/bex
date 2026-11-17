# w4 · m433 — A restart keeps the running shutdown delay, port and pre-deploy command instead of shipping undeployed settings, as Render's Restart keeps the running configuration

**Worker:** worker4 **Goal:** a restart reruns the configuration the instance is running, including its shutdown delay, port and pre-deploy command; a rollback runs the target's; an undeployed save waits for the next deploy and stays reported until then. **Status:** done — t001–t009 done. The 2026-11-16 live run on pin `7aaa8700befa` held for the restart, deploy and rollback; the saved-port read-back fix (bdbea508f) was confirmed live on pin `938c5cb36b49` on 2026-11-17 (t005).

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Release records keep the shutdown delay, port and pre-deploy command each release ran with — **DONE** | 45m | — |
| [t002](done/t002.md) | Restart and rollback run the recorded shutdown delay, port and pre-deploy command — **DONE** | 45m | t001 |
| [t003](done/t003.md) | `undeployedChanges` keeps reporting the saved values through a restart — **DONE** | 40m | t002 |
| [t004](done/t004.md) | ADR004's residual and ADR018's Restart and Rollback rows — **DONE** | 15m | t002 |
| [t005](done/t005.md) | Live check on production: save a new port and shutdown delay, restart, then deploy — **DONE** | 30m | t003, t004 |
| [t006](done/t006.md) | Render parity — **DONE** | 20m | t005 |
| [t007](done/t007.md) | Simplify — **DONE** | 15m | t006 |
| [t008](done/t008.md) | Test coverage — **DONE** | 30m | t006 |
| [t009](done/t009.md) | Closeout — **DONE** | 10m | t007, t008 |

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

## Unblock record (2026-11-17)

- **Affected:** t005, t009. **Source:** verified evidence (release gate), no user decision involved.
- **Finding:** production runs platform pin `938c5cb36b49` (pin commit `a24555d5c`, rolled out 23:11Z on 2026-11-16), which contains bdbea508f (`App.SavedPort`); `bex-api` runs the pinned digest. No gate remains.
- **Next step and check:** repeat the save-then-Restart leg on a fresh `bex-canary` fixture; GraphQL `configuredPort` reads the saved port while `openPorts` reads the running one; delete the service; close with t009.
- **Result:** held the same day (t005 evidence); the milestone closed. REST/MCP `serviceDetails.port` reading the running port is not a Render divergence (Render's API has no such field), so nothing was filed.
