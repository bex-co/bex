# w7 · m44 — Enrich deploy-failure email: commit message + View Logs link + Render-shaped framing

**Worker:** worker7 **Goal:** bex's deploy-lifecycle notification emails carry the context Render's do — the failure email frames the impact ("we encountered an error during the deploy process for `<service>`; your latest changes may not be live"), names the failing **commit message**, and links straight to the deploy's logs ("View Logs" → the deploy-detail page) — so a workspace member who gets the mail can act without hunting through the dashboard. Today bex sends a single terse line (`A deploy of "backend-v2" failed.`) with no commit and no link. The data mostly already exists (the `Deploy` row carries `Commit`/`CommitMessage`/`CommitAuthorAt`; `BEX_DASHBOARD_URL` is wired) but isn't threaded to the mailer. **Status:** done

## Tasks (in order)

| id   | title                                                                                       | est | depends_on |
| ---- | ------------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Thread commit fields + deploy id + dashboard base URL to the notifications service — **DONE** | 40m | —          |
| t002 | Render-shaped deploy email body: impact framing + commit message + View Logs deep link — **DONE** | 40m | t001       |
| t003 | Render parity — verify enriched email vs Render's captured message; update notify-on-fail.md + ADR018 — **DONE** | 20m | t002       |
| t004 | Simplify — `/simplify` over the milestone's diff — **DONE** | 15m | t003       |
| t005 | Test coverage — subject/body assertions for commit + link + framing across started/succeeded/failed — **DONE** | 30m | t003       |
| t006 | Closeout — move to `done/` when the DoD holds — **DONE** | 15m | t005       |

## Definition of done

When a deploy fails (and the member/service notification policy resolves to send):

1. **The email frames the impact** like Render's: a sentence naming the service and stating the deploy didn't complete / latest changes may not be live — not the current bare `A deploy of "x" failed.` line.
2. **The commit message is included** when the failing deploy has one (`Deploy.CommitMessage` — repo-backed builds): the full commit subject/body appears in the body; image-backed deploys (no commit) omit the block cleanly rather than printing an empty label.
3. **A "View Logs" link is present** when `BEX_DASHBOARD_URL` is configured: a deep link to the deploy's detail/logs page (`<BEX_DASHBOARD_URL>/services/<service>/deploys/<deployId>`); the link is omitted (not a broken/half URL) when the dashboard URL is unset, matching the invite email's honest-omit behavior.
4. **Consistency across the three lifecycle kinds**: started/succeeded/failed all render through the shared composer with matching structure (succeeded says "went live", started says "started"), each carrying the commit + View Logs link when available — no jarring rich-failure / terse-success split.
5. **No REST/GraphQL/MCP/dashboard surface changes** — the notification *policy* (who is notified, the `notificationsToSend` states) is unchanged; only the email body/subject the existing path emits is enriched. The parity-ledger Notifications row stays ✅.

## Source + Goal linkage

- **Source:** User request 2026-07-17 — Render's own deploy-failure email (quoted): "We encountered an error during the deploy process for backend-v2. This means your deploy didn't complete successfully and your latest changes may not be live. Commit: `<full commit message>`. View Logs." Explore survey of bex's path found the gap is content-only: `internal/notifications/service.go:274` `deployEmail()` emits one terse line; the `Deploy` store row already carries `Commit`/`CommitMessage`/`CommitAuthorAt` (`internal/store/store.go`) but the reconciler's `DeployNotification` struct (`internal/store/reconciler.go:94`) drops them, and the notifications service never receives `BEX_DASHBOARD_URL` (wired as `deps.DashboardURL` in `cmd/api/main.go`). The deploy-detail route `/services/<service>/deploys/<deployId>` (which shows build/deploy logs) is the confirmed "View Logs" target.
- **Goal linkage:** Render parity + AI-native operability ([docs/ADR008-vision.md](../../../docs/ADR008-vision.md) #2 "basic obs for operation") — a self-contained failure email (what broke, which commit, one click to the logs) is how an operator or agent triages a bad deploy from their inbox. The parity ledger's Notifications row is ✅ on policy/surfaces; this closes the email-content depth gap under it.
- **Expected outcome:** A workspace member who receives a deploy-failure email sees the service, the impact, the failing commit message, and a one-click link to the logs — matching Render's email, without opening the dashboard to find any of it.
- **Why now:** w7 is the live-comparison polish workstream (m41 static create, m42 logs, m43 scaling — same pattern of comparing bex's output to Render's and closing the depth gap); the failure email is the highest-signal notification and today it's the least informative part of the deploy experience. The enabling data already exists on the deploy row and in config, so this is small, self-contained backend work with no schema or surface change.
- **Render parity:** included (t003) — the email is a user-facing surface, and the milestone's whole point is matching Render's captured email; the closing check verifies content against the quote, confirms no REST/GraphQL/MCP/UI drift (email-only), and updates the `notify-on-fail.md` artifact + the ADR018 Notifications row note.
