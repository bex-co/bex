# w2 · m70 — Outbound webhooks: truthful events + Render wire/dashboard parity

**Worker:** worker2 **Goal:** make bex's outbound-webhook events, Render REST route family, delivery behavior, and dashboard agree with Render wherever bex has a truthful source, while preserving the explicitly chosen mint-once secret and documented product non-goals **Status:** done

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Pin Render's current webhook contract and the supported/divergent event matrix — **DONE** | 45m | — |
| t002 | Project existing lifecycle + auto-deploy facts into signed outbound webhooks — **DONE** | 75m | t001 |
| t003 | Emit observed cron-run start/end facts exactly once with terminal status — **DONE** | 75m | t001 |
| t004 | Align create/update semantics: required unique name, enabled, all-events, and HTTPS — **DONE** | 60m | t001 |
| t005 | Align list/read/history envelopes, pagination, time filters, and bounded response evidence — **DONE** | 75m | t001, t004 |
| t006 | Reconcile delivery-attempt count and retry timing with Render's documented maximum — **DONE** | 30m | t001 |
| t007 | Finish dashboard create/list/detail/activity parity over the corrected contract — **DONE** | 75m | t002, t003, t004, t005, t006 |
| t008 | Render parity — **DONE** | 30m | t007 |
| t009 | Simplify — **DONE** | 30m | t008 |
| t010 | Test coverage — **DONE** | 45m | t008 |
| t011 | Closeout — **DONE** | 15m | t009, t010 |

## Definition of done

- The served webhook vocabulary and signed deliveries include `branch_deleted`, `build_started`, `build_ended`, `pre_deploy_started`, `pre_deploy_ended`, `job_run_ended`, `auto_deploy_enabled`, and `auto_deploy_disabled`; every terminal Render event carries `data.status` from its durable source.
- Scheduled and manually triggered cron runs produce one start and one terminal event from observed run state, including `succeeded`, `failed`, or `canceled`; cancel intent alone never masquerades as completion.
- A fixture generated from the pinned 2026-08-16 Render webhook contract passes create, list, get, patch, delete, and event-history requests against bex for every supported non-secret field, including Render pagination and empty-`eventFilter` means all-events semantics.
- The mint-once signing secret remains an explicit, tested security divergence: it is returned only on create and the docs do not claim strict compatibility for secret retrieval.
- A failed delivery is attempted no more than Render's documented maximum, observes the 15-second response budget, sends the third-failure notice, and disables the endpoint only after exhaustion.
- The dashboard's create, list, detail, settings, and activity views consume the corrected contract, use human event labels, expose accurate delivery evidence, and pass typecheck, lint, and meaningful component tests.
- Unsupported Render event families are named in the parity artifact with their source/non-goal rationale; no event is advertised without a durable truthful producer.

## Completed outcome — 2026-08-16

m70 replaces the stale blanket-parity claim with a tested, dated contract. bex now advertises 32 truthful webhook event types: 29 exact overlaps with Render's pinned 67-value API enum plus three documented bex extensions. Existing branch/build/pre-deploy/job facts and discriminated auto-deploy state reach signed payloads; cron start/end comes from observed durable run status, not trigger/cancel intent, and replay is idempotent and batched.

The supported REST family now matches Render's non-secret create/list/get/patch/delete/history contract: required owner/name/enabled/filter inputs, workspace-unique names, HTTPS destinations, empty-filter future-inclusive all-events semantics, sparse updates, repeated/comma-separated multi-owner paging, stable time/status history filters, and bounded UTF-8 response evidence. Delivery uses exactly eight total 15-second-bounded attempts through 32h40m30s, with the existing third-failure notice and exhaustion disable. GraphQL/MCP share core validation and retain richer diagnostics. The dashboard can create disabled/all-event endpoints, uses translated event labels, resolves creator identity, links destinations/status to the right actions, and pages server-filtered activity with expandable evidence.

Exact deliberate divergences remain:

- 38 Render enum values remain unsupported because bex lacks a truthful durable producer or the underlying provider/workflow/preview/disk/edge-cache product is an explicit non-goal.
- The signing secret remains mint-once; Render's shared response schema makes it readable after creation.
- bex uses a fixed 25-endpoint workspace cap instead of Render's plan-specific 1/100 limits.
- GraphQL/MCP endpoint management and rich retry evidence are bex extensions; Render's official MCP has no webhook tools.
- Creator email/name resolution is present, but no creator avatar projection exists.

Verification completed locally: `go build ./... && go test ./...`; `make lint-backend` (zero issues); `yarn typecheck`; `yarn lint` (zero errors, one unrelated pre-existing warning); `yarn test` (308 files, 2,124 tests); `bash -n scripts/webhooks-verify.sh`; and `git diff --check`. Real-Postgres cases for migrations, uniqueness, pagination/time/status filters, batched replay, and two-worker signed delivery run in CI behind `BEX_TEST_DB_URI`; a local disposable Postgres attempt was not claimed because this machine's Docker daemon hung before readiness, and the exact temporary containers were stopped/removed. Live `scripts/webhooks-verify.sh` remains an operator acceptance step requiring a caller-supplied public HTTPS receiver, as production correctly rejects HTTP/private SSRF targets.

Intentional implementation complexity retained after `/simplify`: the handwritten typed documents in `dashboard/src/features/webhooks/api/operations.ts` follow the repository's established fallback while the current GraphQL 17/codegen plugin crashes offline; creator resolution uses the authorized cache-first members query because no narrower projection exists and the broader team hook adds invite reads and polling.

## Source + Goal linkage

- **Source:** User request on 2026-08-16 to compare `dashboard.bex.co/webhooks` with Render and hand the findings to `/pm` for w2. The audit checked current `main`, Render's current webhook docs and public OpenAPI, and the earlier live dashboard capture in `docs/render-artifacts/webhooks-ui.md`. The exact OpenAPI diff found 24 served bex types versus 67 Render API values: 21 shared, 46 API values missing, and three bex-to-API extensions (`branch_changed` plus two Postgres credential events still present in Render's prose guide). Six lifecycle facts were already stored but not projected, terminal status/cron semantics were incomplete, and wire residuals remained after `w3/done/m27`.
- **Goal linkage:** ADR008 pillars 1 and 3: Render-compatible REST/GraphQL plus the same core semantics for MCP and the human dashboard. Webhooks are also an API-first integration primitive for agents, so route-only compatibility is insufficient when events or envelopes differ.
- **Expected outcome:** Render-trained clients and webhook consumers can use bex's supported webhook surface without request-shape adapters; receivers get every truthful existing lifecycle transition exactly once per endpoint with the documented status and retry behavior; the dashboard accurately presents the same contract.
- **Why now:** The parity ledger currently marks outbound webhooks complete and still calls API management a bex-only advantage even though Render now publishes webhook CRUD/history. More importantly, six facts shipped by w7/m66 are silently dropped by the outbound projector. Fixing the evidence path before adding more producers prevents the false-complete claim from hardening into client expectations.
- **Gap/replacement intent:** This does not duplicate `w3/done/m27`. That milestone introduced Render's route names and PATCH verb, but current code still omits required create fields, rejects Render's empty-filter meaning, returns different list/history envelopes, and never wired the later w7/m66 facts. m70 closes those verified residuals and updates the stale documentation.
- **Anti-goal boundary:** Persistent disks, provider maintenance/hardware events, pipeline-minute exhaustion, zero-downtime platform redeploy events, and edge-cache events remain out of scope under `.pm/DO_NOT_DO.md`; this milestone documents them rather than fabricating sources or reopening their products. It does not build a new one-off-job product—`job_run_ended` only projects the durable fact already produced by the existing code. The 2026-07-17 mint-once-secret decision remains in force.
- **Render parity:** Included as t008 because this milestone changes REST, GraphQL, MCP, and dashboard-visible behavior.
