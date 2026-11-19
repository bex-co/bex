# w4 · m504 — A node roll's drain-guard handover records Render's `zero_downtime_redeploy_started` and `_ended`

**Worker:** worker4 **Goal:** every App drain-guard hands over before a node roll or a Cluster Autoscaler scale-down shows one `zero_downtime_redeploy_started` and one `zero_downtime_redeploy_ended` on REST, GraphQL, MCP, webhooks and the dashboard, as Render records its own platform-initiated zero-downtime redeploys. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | The operator records each drain-guard handover on the App's status | 60m | — |
| t002 | Project the handover into service event facts | 40m | t001 |
| t003 | Serve the pair on REST, GraphQL, MCP and webhooks | 45m | t002 |
| t004 | Show the pair in the dashboard's Events feed and webhook picker | 35m | t003 |
| t005 | Correct the docs that call the pair an anti-goal | 25m | t003 |
| t006 | Verify a node roll on the local CAPD cluster | 45m | t004, t005 |
| t007 | Render parity | 20m | t006 |
| t008 | Simplify | 15m | t007 |
| t009 | Test coverage | 30m | t007 |
| t010 | Closeout | 10m | t008, t009 |

## Definition of done

1. On the local CAPD cluster, deleting a guarded `bex-tenant-0` Machine whose node holds the only Ready instance of a web service records, for that service, exactly one `zero_downtime_redeploy_started` with `details.trigger: node_replacement` and one `zero_downtime_redeploy_ended`, readable on `GET /v1/services/{id}/events`, `GET /v1/events/{eventId}`, GraphQL and MCP; a webhook subscribed to both receives both; an HTTP probe through the roll sees no non-2xx answer.
2. A Cluster Autoscaler scale-down handover records the pair with `trigger: node_consolidation` (live, or envtest where the local autoscaler cannot be driven).
3. A disk-attached service records neither; an operator restart or backend re-read records no duplicate.
4. Both types pass `TestRenderEventDetailsMatchTheirMember` against Render's members with no missing fields.
5. `docs/render-artifacts/webhooks-api.md`, `docs/render-artifacts/service-events.md`, ADR018 (Service events and Outbound event webhooks rows), ADR004 § Node rolls and the drain-guard runbook describe the pair as produced; `maintenance_started`/`_ended` stay unsupported.
6. `cd lego/backend && go test ./...`, `make test` and `make lint` in `lego/operator/`, and dashboard `yarn lint` + `yarn test` pass.

## Source + Goal linkage

- **Source:** `/pm-brainstorm one aspect of render parity for w4` (the service events and outbound webhooks aspect), 2026-11-18, item 1; filed by `/pm them to w4`, 2026-11-19.
- **Render reference:** render.com/docs/webhooks, read 2026-11-18: `zero_downtime_redeploy_started` "A Render-initiated zero-downtime deploy started for a service.", `zero_downtime_redeploy_ended` "A Render-initiated zero-downtime deploy completed for a service." The pinned OpenAPI `retrieve-event` members: Started `required: [trigger]` (`trigger: string`), Ended no fields. render.com/docs/notifications sends no email for either.
- **bex today:** drain-guard (w1/m186, `db0066b3a`, 2026-10-26; w1/m187) rolls a single-instance App off a node before CAPI drains it or the autoscaler removes it, by setting `drain-guard.bex.co/evacuated-from` on the pod template (`lego/operator/internal/drainguard/drainguard.go:77`). No backend code reads it, so the handover appears nowhere a tenant can see. `docs/render-artifacts/webhooks-api.md` still files the pair as "Repository anti-goal; document, do not fabricate" and `service-events.md:125` says "No durable bex source exists"; `w5/done/m142/done/t003.md:44` excluded it on 2026-10-25, a day before drain-guard landed, for "no platform-initiated redeploy source".
- **Goal linkage:** ADR008 Render parity first; ADR018 Outbound event webhooks row (its remaining ◐ names the zero-downtime-redeploy family) and Service events row.
- **Expected outcome:** a tenant sees why its instance changed during a node roll, and a webhook receiver written for Render gets the same pair.
- **Why now:** `w4/blocked/m200`'s kubelet node roll and ADR053's node replacements will hand over every single-instance service on `bex-tenant-0` once they run, with no record; the stale "no source" classification is repeated in three documents.
- **Not reopened:** Render's `/maintenance` operations and `maintenance_started`/`_ended` stay a `.pm/DO_NOT_DO.md` non-goal (Maintenance runs). This milestone reports a handover bex already performs.
- **Recommendations, not user decisions:** the trigger values `node_replacement`/`node_consolidation` (Render's vocabulary is uncaptured, `w10/blocked/002`), and ending a transition whose hold expired without a separate outcome (Render's ended member has none).
- **Render parity:** included: the change adds event types to REST, GraphQL, MCP, webhooks and the dashboard.
