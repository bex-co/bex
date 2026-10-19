# w4 · m232 — Keep a protected environment's members behind its admin gate on every move

**Worker:** worker4 **Goal:** no placement write can take a service, datastore or env group out of a protected, isolated or allow-listed environment without that environment's fresh administrator check, and the dashboard says so before a move happens. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](t001.md) | Enumerate every writer that can end an environment membership | 30m | — |
| [t002](t002.md) | Require fresh `can_manage` on the ACL-bearing environment a resource leaves | 60m | w4/m232/t001 |
| [t003](t003.md) | Show each candidate's current environment and protection in Manage resources | 40m | w4/m232/t002 |
| [t004](t004.md) | Render parity | 15m | w4/m232/t003 |
| [t005](t005.md) | Simplify | 10m | w4/m232/t004 |
| [t006](t006.md) | Test coverage | 45m | w4/m232/t004 |
| [t007](t007.md) | Closeout | 20m | w4/m232/t005, w4/m232/t006 |

Seven tasks, 220m (3h40m). One **major** finding: an administrator-only boundary can be left through verbs that check only their own target. This milestone schedules the work only.

## Definition of done

Replay [finding.md](finding.md) with the [recorded probes](evidence.json) in bex-canary as the QA user (admin). Use fresh `qa-<date>-` fixtures, a project with a `production` environment (protected and isolated) and `staging`, plus one Free web service. Delete them all at the end.

- **Developer refusal on every writer (tests, because no live developer exists on Hobby).** A `developerChecker` caller is refused with `ErrForbidden`, and store membership, the isolation label and the inbound-IP layer read back unchanged, for:
  - env `SetServices` on an unprotected environment naming a member of an ACL-bearing one
  - env `SetDatabases`/`SetKeyValues`/`SetEnvGroups` pulling a member out the same way
  - project `SetServices` omitting a member of an ACL-bearing environment, and project `SetServices` of another project naming one
  - `MoveEnvGroup` out of an ACL-bearing environment
  - every further writer t001 lists
  - Each adapter's REST status is `403`, and GraphQL and MCP carry the same refusal.
- **Controls (tests):**
  - An admin succeeds on each path.
  - A developer still moves resources between bare (non-ACL) environments and still adds a resource **into** an ACL-bearing environment.
  - The existing `TestACLBearingMembershipTeardownRequiresCanManage` stays green.
- **Live admin move, now explained.** Staging → Manage resources lists the service with its current environment `production` and a protected/isolated marker. Before Save the dialog says it will move out of `production`. Save moves it (REST `environmentId` = staging).
- **Live guard control.** While the service is in protected `production`, `POST /v1/services/<id>/suspend` without `confirm` → `400 PROTECTED_ENVIRONMENT_CONFIRMATION_REQUIRED`. After the admin move to staging, the same call → `202` (resume afterwards).
- **Checks pass.** Backend `go test ./...` (real-DB gate), dashboard `yarn lint` and `yarn test`.
- **Cleanup.** No `qa-` fixture from the replay remains, and only that run's session is revoked.

The developer path stays a test assertion until a Pro-plan QA workspace with a developer member exists. Record that in the closeout rather than claiming it live.

## Source + Goal linkage

- **Source:** live functional `/qa-find-bugs w4`, pass 9d7581, 2026-10-19 UTC, `muse.env`, workspace `bex-canary`. See [finding.md](finding.md) and [evidence.json](evidence.json). Production was pinned to `f3871ba9fd44`, and research is on main `e572e7977`.
- **Goal linkage:**
  - [ADR024](../../../docs/ADR024-members.md): `admin` alone manages protected environments.
  - [ADR032](../../../docs/ADR032-environments.md): one environment per resource, so reassigning moves it.
  - [ADR063](../../../docs/ADR063-security-review-round9.md) #5 and [ADR084](../../../docs/ADR084-security-review-round22.md) finding 3: ACL-bearing teardown needs a fresh `can_manage` "on every call path".
- **Expected outcome:** protected status, network isolation and environment IP allow-lists can only be removed from a resource by someone allowed to edit them, whichever verb they use. Admins see what a move will do before they make it.
- **Why now:** the round-22 fix closed the direct paths and declared the boundary whole. The side doors are one dialog away in the dashboard (Manage resources on any other environment) and one API call away (`setProjectServices`). Lifting network isolation reopens cross-environment connections to the service.
- **Render parity included:** the fix changes REST, GraphQL, MCP and dashboard behavior. t004 compares Render's documented protected-environment rules for moving resources and records any divergence.
- **Unverified:**
  - The developer-role path live.
  - MCP adapters.
  - Datastore and env-group moves live.
  - Whether project datastore setters clear environment labels (t001).
