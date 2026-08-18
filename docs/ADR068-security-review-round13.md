# ADR068: Security review round 13 — X78KUu disposition

- **Status**: Accepted (2026-08-17)
- **Scan**: codex-security `X78KUu`, repository revision `69cdf9d7` (2026-08-17), 9 findings (1 high, 6 medium, 2 low)
- **Lineage**: thirteenth pass in the ADR028 → ADR045 → ADR055 → ADR056 → ADR057 → ADR060 → ADR061 → ADR063 → ADR064 → ADR066 → ADR067 lineage

## Summary

Seven of nine findings are fixed in place with regression tests; the remaining two are re-confirmed standing residuals (PSL submission, eighth report; digest-pinning inventory, sixth report). No finding was rejected outright.

| # | Finding | Severity | Disposition |
| --- | --- | --- | --- |
| 1 | Generic sandbox exec bypasses the agent-session shell authorization boundary | high | **Fixed** — agent-session sandboxes require fresh `can_view_sensitive` on the session object at the generic exec seam; the binding is signed into exec claims and re-enforced at the gateway |
| 2 | Shared production tenant suffix permits cross-tenant cookie injection | medium | **Accepted residual** — onbex.co PSL submission (eighth report), blocked on operator action |
| 3 | Long-running sandbox exec stays live for hours after revocation | medium | **Fixed** — sandbox-exec joins the redemption re-check + `WithRevalidation` watchdog family (round-9 #6) |
| 4 | kpack builds lack an ephemeral-storage bound | medium | **Fixed** — `ephemeral-storage` 10Gi/16G in the kpack Image build resources, matching the Dockerfile/native bound |
| 5 | General usage permission exposes Stripe billing records to non-billing roles | medium | **Fixed** — `Summary.Billing` attached only under `can_manage_billing` (fresh); estimate stays `can_view` |
| 6 | Two slow static publishes occupy every App reconciler worker | medium | **Fixed** — publish becomes Ensure/Observe + `RequeueAfter` (the ADR060 §D1 shape), no in-reconcile wait |
| 7 | Workspace viewers can retrieve credential-bearing webhook destination URLs | medium | **Fixed** — URL userinfo refused at create/update; non-admin reads get the origin only (`https://host/…`) |
| 8 | Pod-bound model proxy lacks a request/spend budget | low | **Fixed** — cumulative per-session (1000) + per-workspace (5000) exchange budgets, atomic, pre-mint |
| 9 | KeyValue backups execute mutable tooling over plaintext backup material | low | **Deferred** — digest-pinning inventory (sixth report), extends to `busybox:1.37`/`valkey:<tag>` snapshot+compress stages |

## Finding 1 (high) — agent-session sandboxes under the generic exec verb

An agent-session sandbox carries ordinary owner/workspace/regime metadata plus the `bex.co/agent-session` binding, so the **generic** `POST /v1/sandboxes/{id}/exec` (REST/MCP) accepted it with only workspace `can_operate` + ownership — while the repository's own FGA model documents that a shell into that pod class must gate on the session object's `can_view_sensitive` because "a contributor holds can_operate … and could otherwise … read credentials attach never reveals" (`deploy/gitops/authz/model.fga`). A session owner demoted to contributor could exec arbitrary commands into the credential-capable sandbox (Git-write and model proxies reachable from it) through the weaker side door.

Fixed at three layers, mirroring the round-5 finding-4 "fresh check at the privilege sink" class:

1. **bex-api mint gate** (`lego/backend/internal/sandbox/exec.go`): `dialGateway` now reads the resolved sandbox's session label and, when set, requires `AuthorizeFreshOn(ctx, RelCanViewSensitive, agent_session:<id>)` before minting any ticket — the exact relation the dedicated `ags-…` SSH path and agent attach enforce. Ordinary sandboxes are untouched (`can_operate` + ownership as before), and the workspace-admin cross-owner override is not weakened: admins hold `can_view_sensitive` too.
2. **Signed binding** (`internal/sandboxexec`): `Claims.AgentSessionID` is HMAC-signed into the ticket (`ags`), so the gateway re-derives nothing — the session binding travels in the signed claims. Every mint site passes it (the public verb, the trusted Completer's status/transcript/hibernation reads, and the pre-snapshot scrub).
3. **Gateway enforcement** (finding 3's revalidator, shared): redemption and every live-stream tick re-require `can_operate` on the ticket's workspace **and**, when `ags` is set, `can_view_sensitive` on the session object — defense in depth behind bex-api's gate.

The trusted seam is deliberately split: the generic verb's gate applies to **caller-chosen** commands only. `clientSuspend`'s pre-snapshot credential scrub is a platform-fixed command (output discarded, exit code checked), so it now runs through `SystemBufferedExec` → `mintAndDial` directly — a contributor (who may legitimately suspend a session sandbox, the same relation the dedicated stop verb uses) keeps that path while losing arbitrary exec.

Tests: `TestExecAgentSessionSandboxRequiresViewSensitive` (contributor forbidden on a session sandbox they own + ordinary sandbox still allowed), `TestExecAgentSessionSandboxSignsSessionClaimForDeveloper` (developer allowed; ticket carries `ags-one`), `TestSuspendRunsPlatformScrubUnderContributor` (the platform-fixed scrub still runs and is signed with the session binding).

## Finding 3 (medium) — sandbox exec joins the revalidation family

Round-9 #6 wired `sshgateway.WithRevalidation` into native SSH, the web shell, and agent attach — but sandbox exec kept admission-only authorization: after ticket redemption the command ran to disconnect or the 4h `SessionTimeout`, so a membership/role revocation mid-exec took effect only at the next admission. `sandboxsse` now:

- performs a **redemption-time re-check** (`ExecRevalidator`, the agentattach round-6 #11 pattern) before the SSE headers — a ticket whose subject lost the relation inside the mint→redeem window is a clean 403, and the pod is never exec'd;
- wraps the exec context in **`WithRevalidation`** on the shared `BEX_SSH_REVALIDATE_INTERVAL` (default 1m), emitting `Metrics.Authentication("revoked")` and an SSE error event when the watchdog — not the client or the cap — ends the stream.

`ExecRevalidator` holds only the authorization kernel (`*core.Base`), skips platform-internal tickets (`sandboxexec.SystemSubject` — the Completer's reads have no caller identity; the HMAC is their authority), and fails closed on an unreachable checker. The system sentinel and the `agent_session:<id>` object literal each moved to one shared definition (`sandboxexec.SystemSubject`, `agentsession.SessionObject`) so both processes can't drift on them.

Tests: `TestExecRevalidatorRelations` (system exempt; workspace `can_operate`; agent-session `can_view_sensitive`; fresh path; workspace-less caller ticket refused), `TestSandboxExecRedemptionRecheck` (403, executor never invoked), `TestSandboxExecRevocationEndsLiveStream` (mid-stream revocation cancels the executor's context and surfaces the revoked event).

## Finding 4 (medium) — kpack ephemeral-storage bound

The kpack `Image` build resources carried CPU/memory but no `ephemeral-storage`, and `bex-build` has no LimitRange/ResourceQuota by design ("their limits are set directly in the Job spec", `deploy/gitops/base/tenant-quotas.yaml`) — so buildpack steps executing tenant-controlled Git source could fill node-local disk on the shared build nodes, the exact gap w7's codex #3 closed for Dockerfile/native builds. The kpack build resources now set `ephemeral-storage` request/limit to the same `10Gi`/`16G` the build Job's workspace containers carry, so every build flavor (Dockerfile, native, CNB/kpack) enforces the identical disk budget and kubelet evicts the offender rather than the node. A namespace-level quota was deliberately not added: the per-workload bound is the mechanism the codebase already documents, and a `ResourceQuota` on the shared build namespace would couple every tenant's build placement to one aggregate number (deferred as belt-and-suspenders follow-up). Tests: `TestBuildJobResourceLimits` (the Dockerfile/native bounds) and `TestBuildpackImageShapeAndSuccess` in `lego/operator/internal/build/build_test.go` now also assert the kpack Image's ephemeral-storage requests/limits.

## Finding 5 (medium) — billing projection under the billing relation

`monthToDateAt()` authorized `can_view` (viewer and up) and then unconditionally attached `Summary.Billing` — the real Stripe projection (current cost, finalized invoices, named credit grants) that `model.fga` reserves for `can_manage_billing: billing or admin`. Every viewer, contributor, and developer read the workspace's actual spend through REST, GraphQL, and MCP. The projection is now attached only when `mayManageBilling` holds — a raw-checker (no denial audit row per ordinary viewer read, the `isWorkspaceAdmin` precedent), **fresh** decision when the checker supports it, so a just-revoked billing member cannot ride a cached positive. Usage rows and the advisory `EstimatedCost` remain `can_view` for everyone; `Billing` is simply null for other roles (the shape estimate-only deployments already produce). With authz unwired (`BEX_OPENFGA_URL` unset), the prior behavior is kept. Tests: `TestBillingHiddenFromNonBillingRoles` (estimate present + reader never consulted for non-billing roles; billing member sees the sample), `TestBillingRevokedMidCacheWindow`.

## Finding 6 (medium) — publish reconciles without blocking

`publish.Publish()` created the tenant-driven publish Job and **polled it inside the reconcile** for up to ten minutes; with `BEX_APP_RECONCILE_WORKERS=2`, two slow publishes occupied the entire global App controller and delayed unrelated deploy/scale/suspend/delete work across tenants. The publish plane now mirrors the build plane's ADR060 §D1 shape:

- `publish.Ensure(ctx, o) (Observation, error)` is create/get, never a wait: `PhasePublishing` / `PhaseSucceeded` / `PhaseFailed` + message. The Job's own `activeDeadlineSeconds` (10 min) owns the wall-clock bound the blocking loop used to hold — a stuck upload is reaped into `JobFailed`, which the next observe reports.
- `publishStaticRevision` requeues after `publish.ObserveInterval` (3s) while the Job runs and fails the App with `PublishFailed` on terminal failure, byte-identical error surface; `reconcileStaticSite` halts on the in-flight result exactly as it halts on errors.

Tests: `TestEnsureNeverBlocks` (fresh Job reports `PhasePublishing` immediately; complete/failed conditions report their phases), and the envtest direct-publish spec now drives the Job between two reconcile rounds instead of relying on the removed blocking behavior.

## Finding 7 (medium) — webhook destination URLs are capability-bearing

Endpoint create/update are admin-only, but list/get are `can_view` member reads — and `toView` returned the exact stored destination, whose path/query commonly **is** the integration credential (Slack `/services/T000/B000/…`, PagerDuty routing keys, `?token=…`). Two halves, both following the repo-URL invariant the store already enforces (`internal/store/api.go` refuses git-URL userinfo because "the repo string is stored … and echoed verbatim to every workspace viewer"):

- `parseDestination` now **rejects URL userinfo** outright (create/update; existing stored URLs are unaffected).
- Reads project conservatively for anyone without `can_manage` on the endpoint's workspace: `RedactedURL` collapses everything past the origin to `https://host/…` (a bare origin stays bare). The exact URL remains visible on the admin-gated verbs (Create/Update/SetEnabled responses) and to `can_manage` read callers; the delivery Worker reads the stored row, never the projection, so deliveries are unaffected. The admin check is a raw fresh check — no per-viewer denial audit noise, and a just-demoted admin cannot ride a cached positive.

Tests: the userinfo case joins `TestCreateValidatesURLAndEventTypes`; `TestDestinationURLRedactedForNonAdminReaders` (viewer list/get redacted, admin exact, Create response exact).

## Finding 8 (low) — cumulative model-proxy exchange budgets

The ADR062/ADR064 proxy bounds each exchange (concurrency 32/2, request 4 MiB, response 64 MiB, lifetime 2h) — but every bound resets on completion, so nothing stopped tenant code running in a live sandbox from **looping** billable inference for the session's lifetime on the delegated BYO key. Two process-local atomic counters now bound the cumulative number of provider exchanges: `BEX_AGENT_MODEL_MAX_REQUESTS_PER_SESSION` (default 1000) keyed on the verified `<ns>|<session>` pair, and `BEX_AGENT_MODEL_MAX_REQUESTS_PER_WORKSPACE` (default 5000) keyed on the workspace every `<ws>-sandbox` session derives from. Both charge **before the credential mint** — an exhausted budget is a 429 with no mint, no provider hop — and meter through the existing `bex_ssh_limit_rejected_total{scope="model_session_budget"|"model_workspace_budget"}`. `0` disables a dimension (pre-round-13 behavior). A janitor sweeps entries idle >24h so the counter map cannot become its own unbounded state; a gateway restart resets the counts (accepted: this bounds runaway loops, it is not spend accounting). Token- and cost-denominated budgets remain follow-up — they need provider-specific usage parsing on the mint path, which ADR062's per-exchange re-mint makes possible later. Tests: `TestProxySessionRequestBudget` (429 before any mint), `TestProxyWorkspaceRequestBudget`, `TestProxyBudgetDisabled` (byte-identical), `TestProxyBudgetAtomicAcrossConnections` (exactly N succeed under a racing burst).

## Re-confirmed residuals

- **Finding 2 — onbex.co PSL (eighth report)**: unchanged from ADR067 #6 / ADR064 #6 / ADR063 #3 / ADR061 #4 / ADR060 #1 / ADR055 #9. `hostingdomain.ValidateSharedSuffix` correctly detects the unlisted suffix and the manager deliberately continues with a loud warning (hardening it back to fatal was tried and reverted on 2026-08-16, `815e003b` — it made the accepted risk unrepresentable and silently disabled platform hosting). The fix is the operator action: submit `onbex.co` to publicsuffix/list (`.pm/w1/050.md`).
- **Finding 9 — digest-pinning inventory (sixth report)**: the KeyValue backup Job's snapshot (`valkey:<version>`), compress (`busybox:1.37`), and encrypt (`alpine:3.21` + runtime `apk add age`) stages remain tag-addressed over plaintext backup material; the upload image is pinned. Same deferral as ADR067 #8 / ADR066 #7 / ADR064 / ADR063 #12 / ADR061 #1 (ADR055 F7 family): a first-party reviewed backup helper image containing `valkey-cli` + `gzip` + `age`, digest-pinned, removes the runtime package install — tracked with the wider inventory (Dockerfile FROMs, kpack, barman, CNPG).

## Not changed (explicitly)

- The generic sandbox **list/get/lifecycle** verbs keep their existing relations for agent-session sandboxes: reads expose only metadata (no credentials), Suspend/Resume stay `can_operate` (the dedicated surface's stop verb uses the same relation, and the pre-snapshot scrub is what makes suspend safe), Terminate stays `can_create` (stricter than the dedicated Cancel). The bypass was specific to arbitrary **command execution**, and that seam is now the strongest on the surface.
- `BEX_APP_RECONCILE_WORKERS` stays 2. Finding 6's fix removes the worker-occupancy hazard for publishes; source builds still wait synchronously inside a reconcile, which is ADR060 D1's own tracked deferral, not this round's finding.
