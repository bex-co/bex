# CLI sweep 549 — type-incompatible `services update` flags and source switching

**Verdict: no bug filed.** Same isolated QA session, `bex-canary`, HEAD-built CLI at `0d1ad2fcf`, 2026-10-19T11:12–11:17Z. One owned free image web service was created, probed, driven through a saved source switch, and deleted by exact ID. Former public URL returned `404`, service reads returned `404`, and the post-delete list did not contain the fixture.

## Journey

Created `qa-20261019-5b6da0-type549` from `docker.io/ealen/echo-server:0.9.2`, waited for `live`, and confirmed `HTTP 200`. Then ran each update as a separate `services update <id> --confirm -o json` invocation:

| Flag on an image-backed `web_service` | Result |
| --- | --- |
| `--branch qa549-never` | Exit `0`; branch was saved for the service's Git source settings. Events recorded `branch_changed` (`main` → `qa549-never`) and `source_changed`. |
| `--build-command`, `--start-command` | Exit `1`: only supported for native runtimes. |
| `--cron-command`, `--cron-schedule` | Exit `1`: not supported for `web_service`. |
| `--publish-directory` | Exit `1`: not supported for `web_service`. |
| `--repo https://github.com/bex-co/bex` | Exit `0`; saved source became the Git repository plus previously saved branch. `imagePath` disappeared from the service read while the already-live image kept serving. |
| `--runtime node` | Exit `1`: cannot switch runtimes via the CLI; use the API directly or recreate the service. |

The saved Git source did not silently deploy. A subsequent explicit `deploys create` attempted the saved nonexistent branch and settled `build_failed` with `fatal: couldn't find remote ref qa549-never`; the prior image release remained `Running` and continued serving `HTTP 200`. That is a truthful saved-source/manual-deploy flow, not a silent fallback.

`undeployedChanges` stayed absent throughout. That is consistent with its narrower documented meaning: ADR004's ordinary Save-only section compares saved environment/file sources and related runtime configuration, while `releaseRecordSpec` likewise excludes source identity except for the special no-repo image comparison. Source identity is surfaced by the saved `repo`/`branch`, `source_changed`/`branch_changed` events, and deploy records rather than this config-drift flag.

## Cleanup

Deleted exact-owned `srv-dbavm7kso40s73cfs9kg`; former URL `404`, service/deploy reads `404`, and list absence verified.

## Unrun

- A valid image-to-Git source switch through a successful build (the deliberate invalid branch proves the saved source is selected without paying for a full valid switch).
- Git-to-image switch through the same sequential updates.
- Combined `--repo` + `--branch` in one update, and source flags on static/cron/private/worker fixtures.
- Production remained on image pin `f4e38371055f` at sweep time, ahead of the backend/operator fixes in w8/129–131 but behind local `main`; do not generalize from local source alone.
