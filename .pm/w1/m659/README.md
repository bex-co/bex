# w1 · m659 — A deploy's page lists Render's Notices: build cache cleared, rolled back to, synced from, another deploy in progress, and the reserved-port warning

**Worker:** worker1 **Goal:** bex's deploy page carries Render's **Notices** section, the list under the deploy header that says why this deploy looks the way it does, in Render's order and words, with its links, plus Render's free-plan reserved-port warning. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](t001.md) | Record Render's Notices and reserved-port warning in `deploy-detail-page.md`, and map each to bex's facts | 45m | — |
| [t002](t002.md) | Keep the Blueprint sync a deploy came from, and serve the Notices' facts on GraphQL `Deploy` | 1h30m | t001 |
| [t003](t003.md) | The operator keeps a listener on a reserved port that its scan sees, so the deploy page can warn about it | 1h15m | t001 |
| [t004](t004.md) | The deploy page's Notices section and reserved-port warning, in all eight locales | 2h | t002, t003 |
| [t005](t005.md) | The deploy page's pending frame still matches its ready state at 1440 and 390 px | 30m | t004 |
| [t006](t006.md) | Walk the Notices on dev-1 and on production | 1h | t005 |
| [t007](t007.md) | Render parity | 20m | t006 |
| [t008](t008.md) | Simplify | 15m | t007 |
| [t009](t009.md) | Test coverage | 30m | t007 |
| [t010](t010.md) | Closeout | 15m | t008, t009 |

## Definition of done

1. On production, as the QA user, on a disposable web service: a **Clear build cache & deploy** deploy shows "Build cache cleared"; a rollback deploy shows "Rolled back to `dep-…`" linking to the target; a deploy opened by a Blueprint sync shows "Synced from" linking to that Blueprint's sync; a deploy queued behind another shows "Another deploy is in progress" linking to it, plus "To avoid queuing deploys, adjust your overlapping deploy policy." with its link only while the workspace's policy is Wait.
2. A past deploy whose build is no longer stored shows "Rollback is not available because this build no longer exists." The refusal itself stays w8/m156's.
3. A free web service whose only HTTP listener is on 18012 shows Render's warning: "It looks like your service might be listening on port 18012, which is a reserved port for free compute plans. Please configure your service to listen on a port that is not part of our reserved ports: 18012, 18013, 19099."
4. The failure reason (with the step that failed), the cancel reason and the timeout reason sit inside the Notices list, in Render's order, with their wording unchanged.
5. Every notice reads in all eight locales. The deploy page's pending frame matches its ready state at 1440 and 390 px, with no sideways scroll at 390 px.
6. Backend tests cover each new GraphQL field, including its absence. REST and MCP deploy objects are unchanged unless t001 finds the fields in Render's pinned contract.
7. [deploy-detail-page.md](../../../docs/render-artifacts/deploy-detail-page.md) § Notices and ADR018's Deploys rows say what bex does.

## Source + Goal linkage

- **Source:** `/pm-brainstorm one aspect of render parity to w1` (2026-11-26), verified the same day before filing. Render's public dashboard bundle (entry `index-D5V4ZgbE.js`), `DeployContainer` chunk: a `Notices` heading over a list of, in order, the failure reason with its failure step; "Build cache cleared" (`triggerDetails.clearCache`); "Transferred from …"; "Updated to a free compute plan" (`isTransitionToFreeTier`); a system trigger's message; "Rolled back to `<dep>`" (a rollback or API trigger with `targetDeployId`); "Rollback is not available because this build no longer exists." (`ROLLBACK_UNSUPPORTED_BUILD_NO_LONGER_EXISTS`); "Synced from `<Blueprint>`" (an IaC trigger, linking its `iacSourceId` and `iacExecutionId`); the cancel reason; "Another deploy is in progress", then ". To avoid queuing deploys, adjust your overlapping deploy policy." when the policy is `QUEUE_AND_UPDATE`; and "Timed out" with the health-check notice. Above the log, on a free plan, the reserved-port warning when `openPorts` holds an HTTP listener on 18012, 18013 or 19099. No flag gates any of it.
- **bex today (verified 2026-11-26):** `dashboard/src/features/deploys/components/deploy-detail-page.tsx` has no Notices section (`deploy-page-skeletons.tsx:153`: "notices depend on the service, so none is reserved"). A rollback shows only in the header's trigger label ("Rollback to {deployId}", `deploy-header.tsx:188`). A deploy whose build is gone simply loses its Rollback (`deploy-rollback.tsx:85`, w8/m156), with no notice. GraphQL `Deploy` has no `clearCache`, although `deploys.clear_cache` has been stored since w4/413 (migration 0165), and no record of the Blueprint sync that opened it (the `deploys` table keeps none; `trigger: blueprint` only). The operator's port scan drops a listener on a reserved port silently (`listenerSet`, `lego/operator/internal/controller/private_ports.go:396-404`), so `openPorts` can never hold one and no deploy reason names it. Failure, cancel and timeout reasons exist (`failureReasonTemplate`, `cancelReasonTemplate`, `deploys.reason.health_timed_out`) and move into the list.
- **Goal linkage:** Render parity first ([ADR008](../../../docs/ADR008-vision.md#render-parity-first)). The deploy page is where a user reads why a deploy did what it did.
- **Expected outcome:** a user can tell from the deploy page, without the logs, that a deploy cleared the cache, was a rollback of which deploy, came from which Blueprint sync, is waiting for which deploy and why, or will never serve because of a reserved port.
- **Why now:** w1/m646 just rebuilt this page's rollback flow, most facts are already stored, and bex already offers Render's Wait policy (ADR018 § Identity, workspaces & account: Workspace Overlapping Deploy Policy) without ever telling a user why their deploy is queued.
- **Not duplicated (checked 2026-11-26 across every workstream, open, blocked and done):** w8/m156 (blocked) refuses a rollback whose build is gone and hides its button, and does not own the notice; w9/m493's "Build cache cleared" is a build-log line, not this notice; w3/125 is the service-level crash and billing notices; w1/159 adds an avatar to the same header's trigger line, so coordinate edits to `deploy-header.tsx`.
- **Out of scope, recorded in t001:** "Transferred from" (bex has no cross-workspace service transfer); the `github_incident_deploy_banner` flag (an incident switch, not a feature).
- **Render parity task:** included (t007), because the milestone adds GraphQL fields and dashboard UI.
