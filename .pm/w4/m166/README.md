# w4 · m166 — Loop2: Docker Command for image-source services

**Worker:** worker4 **Goal:** an image-source web/worker service can set its container command from the dashboard, unblocking images whose default CMD does not serve **Status:** todo

## Tasks (in order)

| id   | title                                                        | est | depends_on |
| ---- | ------------------------------------------------------------ | --- | ---------- |
| t001 | Create wizard: Docker Command field for the image source     | 1h  | —          |
| t002 | Settings: Docker Command editor for image-backed services    | 1h  | w4/m166/t001 |
| t003 | Render parity across create/settings/API surfaces            | 30m | w4/m166/t002 |
| t004 | Simplify the touched dashboard code                          | 30m | w4/m166/t003 |
| t005 | Test coverage for image-service commands                     | 1h  | w4/m166/t003 |
| t006 | Closeout                                                     | 15m | w4/m166/t005 |

## Definition of done

- The New Service wizard shows a Docker Command field when the Existing Image tab is selected (non-cron types), and the created App carries it as `spec.startCommand`.
- An image-backed web/worker service's Settings page shows an editable Docker Command row that round-trips through `setStartCommand` and redeploys.
- No API/operator change: `bex-api` create/update already accept `startCommand` beside `image`, and the operator already projects it onto the container.

## Source + Goal linkage

- **Source:** infinite `/qa-find-bugs` loop2 2026-10-04 UTC (muse.env, journeys 5,9-15). Live evidence: `hashicorp/http-echo:0.2.3` web service crash-looped with exit 127 (`Missing -text option!`) and an `alpine:3.20` worker exited 0 on loop — both unrescuable because no dashboard surface sets the command. Screenshot: `.playwright-mcp/qa-worker-crashloop-no-command.png` (deleted-fixture evidence, not committed).
- **Goal linkage:** Render parity for service creation/settings (ADR018); unblocks the whole class of prebuilt-image deploys whose default CMD needs args.
- **Expected outcome:** image deploys that need command args (echo servers, one-shot CLIs, alternate entrypoints) become deployable from the dashboard instead of crash-looping with no recourse.
- **Why now:** QA proved the backend + operator already support it end to end — the dashboard is the only missing surface, so this is a small, safe, high-leverage gap close. Render parity task included (t003): user-facing create/settings surface with REST/GraphQL/MCP equivalents to check.
