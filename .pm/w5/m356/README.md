# w5 · m356 — In the zh dashboard, a deploy's port hints and a build's queued cause stay English, and the hint comes and goes between passes

**Worker:** worker5 **Goal:** the health-check stall's port hints, a queued build's cause and the deferred restart's wait read in the dashboard's language, and the stall text holds still while the deploy waits; REST and MCP keep Render's English byte for byte. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](t001.md) | Types: templates for the port hints, the fallback-HTTP hint, the build's queued causes and the gRPC stall; a parser that reads an appended tail sentence by sentence | 60m | — |
| [t002](t002.md) | Operator: hints and queued causes format through the templates; the stall keeps the last port-scan verdict, so its text holds between passes | 45m | t001 |
| [t003](t003.md) | bex-api: GraphQL `DeployReason.parts` for a hint or cause that parses | 30m | t001 |
| [t004](t004.md) | Dashboard: render the parts, en and zh keys, `{wait}` as a localized duration | 45m | t003 |
| [t005](t005.md) | Live verification in zh on owned services | 20m | t002, t004 |
| [t006](t006.md) | Render parity | 10m | t005 |
| [t007](t007.md) | Simplify | 15m | t006 |
| [t008](t008.md) | Test coverage | 30m | t006 |
| [t009](t009.md) | Closeout | 5m | t007, t008 |

Nine tasks, 260m (4h20m).

## Definition of done

- In `bex-canary`, with the dashboard in zh, a web image service whose server binds only 127.0.0.1 shows its stall on the deploy header entirely in Chinese, the port hint included, and three reads 5 s apart show the same text.
- A deploy held behind the workspace's concurrent-build limit shows its queued cause in Chinese on the deploy header and the Deploys list.
- A restart that waited for a deploy and timed out shows its wait as a Chinese duration, not "3h0m0s".
- REST `GET /v1/services/{id}/deploys` and MCP return the same English sentences as before; the writers' tests pin them byte for byte.
- An appended sentence no template words still shows, in English, after the worded ones.

## Source + Goal linkage

- **Source:** `/pm-brainstorm for 残留问题 for w5` (2026-11-12), item 1, from the residuals w4/m336 recorded (its t007 Simplify and t008 Test coverage records, and the review of `b46178706`).
- **Evidence:**
  - m336 keeps the stall's code with a `{hint}` tail, but the tail stays English. Every tail sentence is bex's own:
    - `imagePortStallHint`'s three variants (`lego/operator/internal/controller/app_controller.go:6124-6133`) and the fallback-HTTP hint;
    - `detectBoundPort`'s four variants and `noOpenPortMessage`, with its "it listened on port N before" clause (`port_detect.go:138-210`);
    - `detectPrivatePrimary`'s two (`private_ports.go:233`, `:250`).
  - A queued build's cause is English as well: "`%s has %d/%d concurrent builds active; waiting for a slot`" (`app_controller.go:1563`), "a concurrent dispatch took the last build slot; waiting for a slot", "waiting for build capacity: …", and the datastore waits from `prepareSourceBuildEnv`.
  - `restart_wait_timed_out` prints Go's "3h0m0s" inside the zh sentence.
  - The port scan runs once per 15 s (`dueUnreadyScan`), while an in-flight rollout requeues every 5 s, so the hint appears on about one pass in three, and the stall text changes between reads in both languages.
- **Goal linkage:** the dashboard speaks its user's language (w4/m329, w4/m336). Render parity first keeps the API's English unchanged.
- **Expected outcome:** the stall a zh owner meets most (an image that listens nowhere the platform can reach) reads entirely in Chinese, and the text stops flickering.
- **Why now:** m336 shipped the mechanism and waits for its live replay (w4/m336 t005), which meets exactly this mixed, flickering text. w4/615, still open, changes how `PORT` is chosen and may reword these hints. If it lands first, t001's templates follow its sentences; if this lands first, 615 changes the templates with them.
- **Render parity:** included (t006). The change touches GraphQL and the dashboard. REST and MCP keep Render's English, and the parts are a GraphQL extension, as m329's codes are.
- **Dedupe:** w4/m336 (blocked on its release gate) coded the main sentences; w4/600 (zh absolute timestamps) is a different string family. Kubelet text that is not bex's stays English by design.
