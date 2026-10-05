# w4 · m171 — A deploy cut off by a user suspend closes canceled, not as a false health-gate failure

**Worker:** worker4 **Goal:** suspending a service while a deploy is in flight closes that deploy `canceled` straight away, with a cancel reason naming the suspend. It is never left "In Progress" for 18 minutes and then failed with a health-gate line that blames the user's code. **Status:** todo

## Tasks (in order)

| id   | title                                                              | est | depends_on                 |
| ---- | ------------------------------------------------------------------ | --- | -------------------------- |
| t001 | Close the open deploy `canceled` when the user suspends            | 1h  | —                          |
| t002 | No failure notifications, webhooks or events for a suspend-cancel  | 30m | w4/m171/t001               |
| t003 | Render parity across deploy read surfaces                          | 30m | w4/m171/t001, w4/m171/t002 |
| t004 | Simplify changed code                                              | 20m | w4/m171/t003               |
| t005 | Test coverage for shipped behavior                                 | 45m | w4/m171/t003               |
| t006 | Closeout                                                           | 15m | w4/m171/t005               |

## Definition of done

Replay on an owned Free image service (`traefik/whoami:latest`, port 8080, `WHOAMI_PORT_NUMBER=8080`; deleted afterwards):

- **Setup:** deploy it to Live. Trigger a new deploy (deploy hook or Manual Deploy), and within about 10 s suspend through Settings → Suspend.
- **Within 30 s of the suspend:** REST `GET /v1/services/<id>/deploys?limit=1`, GraphQL `deploys(serviceId:)` and MCP agree that the in-flight deploy is `canceled`, with `cancelReason` naming the suspend, no `failureReason`, and `finishedAt` set.
  - Today: it stays `update_in_progress` for 18 min, then closes `update_failed` with "the deploy did not become healthy within the health-gate window; check the service logs" (`dep-db1f20pj8bls738hh1pg`, created 00:36:19Z, finished 00:54:37Z).
- **Dashboard:** the header no longer pairs "Service Suspended" with "Latest deploy: In Progress", and the deploy list shows the row as Canceled with no Cancel button.
- **Resume:** the service returns to the prior Live release (w6/m147 / w1/m172 restore) and the canceled row stays canceled.
- **Controls:** a deploy that genuinely fails its health check (start command `./qa-missing-binary` on a native service) still closes `update_failed` with its real diagnosis (w6/m147). A suspend with no deploy in flight changes no deploy row.

## Source + Goal linkage

- **Source:** infinite `/qa-find-bugs` loop9 2026-10-04 UTC (muse.env, `bex-canary`), journey 3. On fixture `srv-db1f1d3ffk7s73ag80a0` (deleted):
  - Create went Live, then two hook-triggered deploys correctly superseded each other (newer wins by `createdAt`).
  - Hook regenerate: the old URL returned 404 and the new one 200. A hook on a suspended service returned 409 "is suspended".
  - The bug: the hook deploy in flight at suspend time went as described above.
  - Evidence (local): `.playwright-mcp/qa-suspend-midrollout-1.png` (header "Service Suspended" + "Latest deploy: In Progress", Cancel button live).
- **Root cause:**
  - `apps.Service.Suspend` (`lego/backend/internal/apps/service.go:3514` → `setSuspended`) only flips `spec.suspended`. The operator then parks the App, and w6/m147 t002 puts the served template back.
  - The control-plane reconciler (`lego/backend/internal/store/reconciler.go`, around 700) deliberately keeps the open row and its stall diagnosis while Ready carries a park reason ("A park (auto-hibernate, suspend) says nothing about the rollout", `:706`). No verdict ever arrives for a release that will never roll, so `deployCloseFailureReason` (`:813`) falls through at the 18-minute gate to `timedOutDeployReason` (`:1533-1540`).
  - The w6/m147 rule is right for an **auto**-hibernate park (deferred so the verdict arrives), but a **user** suspend is an explicit end to the rollout.
- **Goal linkage:** ADR004 truthful deploy state; ADR008 reliable hosting.
- **Expected outcome:** a suspend cancels the rollout it interrupts, and every surface says so at once. Users stop seeing failed deploys, and (pending t002) failure alerts, for code that never failed.
- **Why now:** suspend is a primary lifecycle action, and a deploy-then-suspend is a normal sequence (deploy hooks from CI, autosuspend scripts). Each one manufactures a false `update_failed` 18 minutes later. Render parity task INCLUDED (t003): REST/GraphQL/MCP/UI deploy surfaces change.
- **Unverified:**
  - Whether the false `update_failed` fired a `deploy_ended` failure webhook, a notification or a `server_failed` event (t002 must check).
  - Render's exact behavior: believed to cancel the in-flight deploy on suspend; t003 confirms.
  - The same shape for a native/git build interrupted mid-build (only an image deploy was probed).
- **Severity:** major.
