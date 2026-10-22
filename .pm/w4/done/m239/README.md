# w4 · m239 — Linking an env group to an Auto-Deploy-off service waits for the owner's deploy

**Worker:** worker4 **Goal:** when a service has Auto-Deploy off, linking, unlinking or deleting an environment group never ships a release on its own. The link waits, exactly as the API's `pendingServiceIds` already says, and reaches the service with the owner's next deploy. That deploy is recorded and reaches a terminal state on its own merits. Today the link changes the operator's release identity, the site is rebuilt and republished with no deploy row, and the next manual deploy is stranded at `created` and then recorded as `canceled`. **Status:** done 2026-10-22 — live DoD replayed on pin `51e6dc76c8fe` ([t007](done/t007.md))

## Tasks (in order)

| id              | title                                                                                     | est | depends_on                 |
| --------------- | ----------------------------------------------------------------------------------------- | --- | -------------------------- |
| [t001](done/t001.md) | Stage a gated group link so it does not change the release identity until the next deploy — **DONE** | 60m | —                          |
| [t002](done/t002.md) | Blast radius: unlink, group delete and secret-file links on gated services, every type    — **DONE** | 45m | w4/m239/t001               |
| [t003](done/t003.md) | No rowless release: a release the operator adopts always has a deploy row                 — **DONE** | 45m | w4/m239/t001               |
| [t004](done/t004.md) | Render parity                                                                             — **DONE** | 20m | w4/m239/t002, w4/m239/t003 |
| [t005](done/t005.md) | Simplify                                                                                  — **DONE** | 15m | w4/m239/t004               |
| [t006](done/t006.md) | Test coverage                                                                             — **DONE** | 40m | w4/m239/t004               |
| [t007](done/t007.md) | Closeout — **DONE** | 20m | w4/m239/t005, w4/m239/t006 |

Seven tasks, 245m (4h05m). One **major** finding, recorded with its complete timeline and probes in [finding.md](finding.md). This milestone schedules the work only.

## Definition of done

Replay [finding.md](finding.md) on production (`https://dashboard.bex.co`) with fresh owned fixtures:

- A static site from `https://github.com/bex-co/bex`, branch `main`, `rootDir: examples/static-site`, `publishPath: .`, `autoDeploy: "no"`, build command `printf "grp=%s\n" "$QA_GRP" > v.txt`.
- Once it is Live with `GET /v.txt` = `grp=`, a group with `QA_GRP=one` linked to it.

The DoD then holds:

- **The link waits.**
  - `POST /v1/env-groups {…, serviceIds:[site]}` still answers `pendingServiceIds:[site]`.
  - For 5 minutes afterwards, `GET /v.txt` keeps answering `grp=`, `GET /v1/services/{id}/deploys` gains no row, and the service's `revision` does not move.
  - No `server_failed` or `server_available` event appears.
- **The next deploy carries it and records it.**
  - `POST /v1/services/{id}/deploys {}` goes `created → (queued →) build_in_progress → live` with non-null `startedAt`/`finishedAt`, plus `build_started`, `build_ended` and `deploy_ended succeeded` events for that deploy id.
  - Then `GET /v.txt` = `grp=one`.
  - No other row changes to `canceled`.
- **The same holds for a second group linked while a deploy is already open,** the 06:09 case in the finding:
  - The open deploy finishes with whatever it was building.
  - The link waits for the next deploy, and no row is stranded at `created`.
- **Auto-Deploy on is unchanged.** The same link on a twin site with `autoDeploy: "yes"` opens a recorded deploy and serves `grp=one`, exactly as today's rolled path does. `pendingServiceIds` is absent.
- **Checks pass:** backend `go test ./...` and operator `make test` and `make lint`. Fixtures deleted (by-id reads 404), and only this run's session is revoked.

- **Web and unlink.** The same holds for a repo-backed web service with Auto-Deploy off (finding follow-up): linking a group and unlinking it each leave `revision` and the instance unchanged, and add no deploy row, until the next deploy. If unlink is decided to roll immediately (t002), it opens a recorded deploy instead.

t002 verifies the private, worker and cron services and the group-delete path as work. The hunt did not probe them live, so they are not asserted here.

## Source + Goal linkage

- **Source:** live `/qa-find-bugs w4` loop run s3g6, 2026-10-21 UTC, `muse.env`, workspace `bex-canary`, pin `9cace6b6e25d`. Probes, timeline, root cause and controls are in [finding.md](finding.md). Local evidence (gitignored): `.playwright-mcp/qa-deploys-s3g6-stuck-created.png` (deploy B at `Created` while its page shows a finished build).
- **Goal linkage:**
  - ADR008: trustworthy deploys.
  - ADR004: a deploy row records every release, and statuses tell the truth.
  - ADR029: static publication.
  - ADR032: environment groups.
  - ADR018: environment-group and Auto-Deploy parity rows.
  - Render: an environment-group change deploys only services with Auto-Deploy on.
- **Expected outcome:**
  - An owner who turned Auto-Deploy off decides when every release ships.
  - Every served release has exactly one deploy row that tells its real story.
  - The API's `pendingServiceIds` promise becomes true.
- **Why now:**
  - It is silent: a change goes live with no record, and no surface reports it.
  - It corrupts deploy history: a deploy whose output served is labeled `canceled`.
  - It shares the stranded-row mechanism with `w8/m117` (blocked on its live repro), so fixing the trigger here also removes a second path into that state.
- **Render parity included:** REST/GraphQL/MCP env-group link writes and the dashboard's "pending" copy must keep agreeing with the new staging behavior.
