# ADR069: Security review round 14 — 4pPvyl disposition

- **Status**: Accepted (2026-08-17)
- **Scan**: codex-security `4pPvyl`, repository revision `dfa381bb` (2026-08-17), 6 findings (1 high, 1 medium, 4 low)
- **Lineage**: fourteenth pass in the ADR028 → ADR045 → ADR055 → ADR056 → ADR057 → ADR072 → ADR061 → ADR063 → ADR064 → ADR066 → ADR067 → ADR068 lineage

## Summary

Five of six findings are fixed in place with regression tests; the sixth is the standing PSL residual, now on its **ninth** report. The high finding is the first _authentication_-layer break the lineage has produced since round 3 — every prior high was an authorization-sink or isolation gap. Nothing was rejected.

| # | Finding | Severity | Disposition |
| --- | --- | --- | --- |
| 1 | OIDC-only consent grants a dynamic client the user's full permissions | high | **Fixed** — a control-plane capability scope (`bex.api`) is required on an API-audience human token, enforced at consent, introspection, and discovery |
| 2 | Tenant serving, cron, and pre-deploy workloads lack ephemeral-storage bounds | medium | **Fixed** — `ephemeralStorage` becomes the compute ladder's third Guaranteed axis; namespace quota + LimitRange gain the aggregate ceiling |
| 3 | Shared onbex.co tenant hosts permit cross-tenant cookie tossing | low | **Accepted residual** — onbex.co PSL submission (ninth report), blocked on operator action |
| 4 | Webhook transport errors disclose exact capability URLs to members | low | **Fixed** — `SanitizeDeliveryError` collapses `*url.Error` to its origin at the write seam; reads scrub legacy rows |
| 5 | KeyValue backups execute mutable images and packages with credentials | low | **Fixed (residual deferred)** — every image bex itself chooses is now digest-pinned; the tenant-chosen version tag and `apk add age` remain the deferral |
| 6 | Webhook failure emails can disclose a replacement URL to a removed creator | low | **Fixed** — recipient resolved from current admin membership, and the body never carries the exact destination |

## Finding 1 (high) — scope discipline: an API-audience human token must ask for the API

This is the sibling of the round-7/w1-m67 **audience** hole, one field over. That fix established "a token minted for another resource must not authorize this one". It left unasked: what if the client requests exactly the right resource — and nothing else?

The full path, all in configured production code:

1. Hydra has **Dynamic Client Registration enabled** (`deploy/gitops/base/values/hydra.values.yaml`) because the MCP authorization spec requires it — an agent self-registers a public PKCE client with no operator involvement. That is intended and stays.
2. Such a client requests `audience=https://api.bex.co/mcp` with `scope=openid offline_access`. The dashboard consent provider granted `requested_scope` and `requested_access_token_audience` **verbatim** — so the user saw a screen that honestly said "openid, offline_access" and approved it.
3. bex-api's introspection decoded `sub`, `client_id`, `aud`, `iss`, `exp` — **not `scope`**. The audience check passed (the client asked for this resource, so `slices.Contains` succeeded).
4. `Base.authorize` then checked `user:<subject>` against OpenFGA. The OAuth client is invisible at that point, so the token carried **every** REST/GraphQL/MCP action the victim's subject may perform: env-var reveal, SQL execute, API-key mint, workspace delete.

The delegation was real, but it was never displayed. A user approving "openid" was approving their entire control plane.

**Fix — one scope, three enforcement points.** The vocabulary is `bex.api` (`BEX_OAUTH_API_SCOPE` on bex-api, `OAUTH_API_SCOPE` on the dashboard; same default, and they must match):

- **Introspection, the fail-closed backstop** (`lego/backend/internal/api/auth.go`). The decoder gains `Scope`; a **human** token whose `aud` contains the configured resource must carry the scope, else the credential introspects inactive → 401, the same shape as any other unacceptable credential. Two exemptions, both principled and both pre-existing:

  - **Machine (`client_credentials`) tokens** — API keys. Their authority is the workspace binding, not a user consent; there is no consent screen to have been misleading. Exempt unconditionally, exactly as under the empty-`aud` rule.
  - **`bex.co/platform-client`-marked clients** — the same Hydra client record, the same TTL-cached lookup, the same marker `scripts/auth-bootstrap-client.sh` stamps that the audience rule already trusts. bex-mobile requests the audience with identity-only scopes today; the marker covers it until its request adds the scope (the bootstrap script now writes `openid offline_access bex.api` for both platform clients, so a re-run closes even that).

  The lookup is paid **only** where the exemption is actually needed — a compliant client that carries the scope never costs a Hydra round trip, and the test asserts that.

- **Dashboard consent, the up-front refusal** (`dashboard/src/common/server-fn/hydra-consent.ts`). On both human-decision paths (GET render and POST decision), a request carrying an access-token audience without the API scope is refused with a 400 naming the scope. The flow fails at consent — with a message the client's developer can act on — instead of minting a token the API will 401. Additionally the grant is now **intersected with a recognized vocabulary** (`openid offline_access profile email bex.api`): arbitrary scope strings a client requests are dropped from the grant rather than rubber-stamped through a user's approval.

- **Discovery** (`lego/backend/internal/api/server.go`). The RFC 9728 metadata advertises `scopes_supported: ["bex.api"]`. The MCP authorization spec tells discovery-driven clients to request a resource's advertised scopes, so a conforming client asks for it with no configuration — which is why this control arms **by default** (unlike `BEX_OAUTH_REQUIRE_AUDIENCE`, which shipped off and stayed off for three rounds).

**Why default-on is safe here.** The exemption class is exactly the class the audience flag's marker already identifies, and the effect on the official Render CLI is nil — its device-flow tokens carry no audience, so the rule never arms for them. `BEX_OAUTH_API_SCOPE` remains overridable per deployment; empty resolves to `DefaultAPIScope`, and an unset `BEX_OAUTH_RESOURCE` leaves the rule inert (byte-identical prior behavior for a deployment with discovery off).

**What this is not.** It is not a per-operation least-privilege scope matrix (read-only vs. write vs. mint-gated scopes). A client that honestly requests `bex.api` and a user who approves it still delegate the user's full authority — ordinary OAuth consent semantics, now truthfully displayed and explicitly requested. The operation-level matrix, and the audit of client-id/subject/audience/scope on each authorization decision, are the recorded follow-ups; they touch every adapter and belong on the board, not in a remediation round.

Tests (`lego/backend/internal/api/r14_scope_test.go`, `dashboard/src/common/server-fn/__tests__/hydra-consent.test.ts`): the third-party identity-scoped audience token is refused and the client record **is** consulted; the four legitimate caller shapes (compliant third party, platform-marked client without the scope, `client_credentials` API key, audience-less device flow) all pass with the Hydra lookup paid only where needed; the rule is inert with no resource or no scope configured; the parser rejects substring and prefix near-misses (`bex.api-readonly`); consent refuses on both GET and POST, exempts a platform-marked client, and honors an `OAUTH_API_SCOPE` override on both sides.

## Finding 2 (medium) — ephemeral storage becomes the third Guaranteed axis

`resourcesForTier` returned CPU and memory only, and every tenant execution mode drinks from it: serving Deployments (`deployment_projection.go`), the CronJob template and one-off `runAt` Jobs (`cronPodSpec`), and pre-deploy commands (`predeploy.Options.Resources`). The namespace `ResourceQuota` and `LimitRange` had the same blind spot — PVC quotas cannot see the writable layer. Tenant code controls what it writes, so `dd if=/dev/zero of=/tmp/x` in a pre-deploy command (or a runaway log) ran to node `DiskPressure`, and the eviction fell on whichever co-tenant pods the kubelet chose. The mechanism was demonstrably available: the KeyValue backup job already bounds its own EmptyDir and ephemeral storage.

Fixed at both layers, with the catalog as the single source:

- **`lego/types/tiers`** gains `ComputeTier.EphemeralStorage`, populated across the ladder (1Gi free → 32Gi pro-ultra, scaling with the tier) and **validated at parse time** — a rung added without a parseable quantity fails catalog load, so it cannot ship unbounded. `Resources()` returns it as a third value; the two callers (`resourcesForTier`, `autoscale.tierLimits`) were updated with the compiler's help.
- **`resourcesForTier`** writes the quantity into both requests and limits, preserving Guaranteed QoS on all three axes. Because it is the one helper every tenant execution mode already used, serving, cron, one-off, and pre-deploy were fixed by that single change — the finding's own preventive control ("make a single resource helper responsible for CPU, memory, and ephemeral storage for every tenant execution mode") was already the shape of the code; only the third axis was missing.
- **`quotaForPlan` / `baseLimitRange`** (`lego/backend/internal/store/namespaces.go`) add `requests.ephemeral-storage` / `limits.ephemeral-storage` (paid 2Ti/4Ti, free 100Gi/200Gi) and LimitRange `defaultRequest` 1Gi / `default` 10Gi / `max` 32Gi, so a **tierless** container (a hand-applied App CR with no `spec.tier`) gets a bound too instead of inheriting none. The aggregates are sized to hold each plan's worst case with headroom — paid ≈ 50 CPU-requested pro-ultra pods × 32Gi ≈ 1.6Ti < 2Ti — so the quota is defense in depth, not a second binding constraint. The legacy shared `deploy/gitops/base/tenant-quotas.yaml` gets the matching keys for the pre-namespace-isolation path.

The kpack/Dockerfile/native build Jobs already carry their own bounds (round-13 #4 finished that inventory); this closes the serving-side half.

Tests: `TestComputeEphemeralStorageBoundedEveryTier` (every rung carries a positive parseable quantity — a new rung without one fails here), catalog-load failure cases for a missing and an unparseable `ephemeralStorage`, the byte-for-byte ladder assertion extended to the disk column, `tier_resources_test.go` for the requests==limits projection, and `datastore_quota_test.go` for the quota/LimitRange keys.

## Finding 3 (low) — onbex.co is not a private Public Suffix (ninth report)

Unchanged and unchanged in disposition. `hostingdomain.Suffix` correctly classifies `onbex.co` as an ordinary registrable domain, and both the manager and static server log the warning and continue — deliberately, because failing closed would mean serving no tenant apps at all. A sibling tenant can set `Domain=onbex.co`, and a victim tenant app using a colliding non-`__Host-` cookie name is exposed.

The fix is not a code change: it is a Public Suffix List submission for `onbex.co`, which requires operator action outside this repository (`.pm/w1/050.md`). Platform-owned cookies already use `__Host-` and are unaffected. Re-confirmed here for the ninth consecutive round; the tripwire (`ErrUnlistedSharedSuffix` is classified, logged, and tested) stays.

## Finding 4 (low) — delivery evidence stops carrying the capability

Go's `http.Client` returns `*url.Error`, whose `Error()` embeds the **full request URL**. The worker persisted `err.Error()` straight into `webhook_deliveries.last_error`, and `ListDeliveriesFiltered` returns `LastError` under `RelCanView` — so an ordinary member reading delivery history saw `Post "https://hooks.example/services/B000/XXX?token=…": dial tcp: …`, i.e. exactly the destination that round-13 #7 had just finished reserving for admins (`RedactedURL`) on the endpoint read. One control was added and its evidence channel kept leaking around it.

Fixed at the write seam and, for already-written rows, at the read:

- **`SanitizeDeliveryError`** (`webhooks/service.go`) unwraps `*url.Error` with `errors.As` and re-renders it as `<op> "<origin>/…": <inner>` — the URL collapses through the same `RedactedURL` helper the endpoint view uses, while the inner transport error (dial, TLS, timeout) is preserved verbatim because it is diagnostic and URL-free. The result is capped at 512 bytes so no error shape can smuggle a payload in bulk into a member-facing display field.
- **`scrubDeliveryEvidence`** collapses any literal occurrence of the endpoint's stored URL in evidence read back from the database. New rows never contain it; this is a migration-time scrub for rows written before the seam existed, applied in `toDeliveryView` (the single projection every REST/GraphQL/MCP read routes through) and in the failure email.

Tests: origin-only rendering for a `*url.Error` carrying a sentinel path+query token, byte-bounding of arbitrary error text, an end-to-end worker attempt against an unreachable destination asserting the sentinel is absent from what is persisted, and a legacy-row read asserting the scrub.

## Finding 5 (low) — the backup pipeline's fixed images are pinned

The KeyValue backup CronJob runs three stages over the plaintext RDB: `snapshot` (holds `REDISCLI_AUTH`), `compress`, and optionally `encrypt` before a digest-pinned AWS CLI uploader (which holds the S3 credentials). Only the uploader was pinned. A retagged `busybox:1.37` or `alpine:3.21` became code inside a pod that reads the plaintext backup, and `alpine` also `apk add`s `age` at job runtime.

Fixed for every image reference **bex itself chooses**:

- `busybox:1.37` and `alpine:3.21` (the age stage) are now digest-pinned.
- `kvDefaultImage` — the Valkey image used when `spec.version` is empty, which is both the serving datastore and the snapshot stage — is digest-pinned. Note this bumps the resolved bytes once at rollout, so the KeyValue StatefulSets on the default version roll; that is a deliberate one-time consequence of pinning, not a regression.

**Residual, deferred.** Two pieces stay unpinned and are the standing digest-pinning inventory item (seventh report, ADR060 §D7 lineage):

- A snapshot stage whose image came from an **explicit tenant-chosen `spec.version`**. bex cannot pre-resolve a digest for a major it has not seen; pinning that requires the internally-built, signed toolchain image ADR060 §D7 specifies.
- `apk add --no-cache age` at job runtime. Same fix, same deferral: build `age` into a pinned, signed internal image.

`TestKeyValueBackupFixedImagesAreDigestPinned` now asserts the split directly — every non-snapshot stage must carry `@sha256:` in both the plain and age-encrypted job shapes, the **default-version** snapshot must be pinned, and only the explicit-version snapshot is exempt. The exemption therefore cannot quietly widen back over the default path.

## Finding 6 (low) — failure notices follow current authorization, not provenance

`webhook_endpoints.created_by` is immutable; `url` is not. An admin can replace the destination long after the creator was removed from the workspace or demoted. The worker's failure notice looked up the **creator's** email with no membership check and put the **current exact URL** in the body — so a removed creator could be mailed a capability a different administrator introduced. Email is an unauthenticated channel, which makes it the worst place for the value round-13 #7 restricted to authenticated admin reads.

Fixed on both halves, in the order that matters:

- **Recipient from current state.** `notifyFailure` first calls `SubjectIsWorkspaceAdmin(tenantID, createdBy)` — a direct `tenant_members` check for the admin role. Non-member, non-admin, and unanswerable (query error) all skip the notice, fail-closed. The check runs **before** `ClaimWebhookFailureNotice`, so a skipped notice does not consume the once-per-window suppression claim and burn the notification for whoever should have gotten it.
- **Body never carries the destination.** Both the 3rd-failure and the final disable notice now render `RedactedURL(d.URL)`, and the quoted `LastError` goes through `scrubDeliveryEvidence` (finding 4's helper). The endpoint's name and redacted origin are enough to act on from an email; the exact URL stays behind the authenticated admin surface.

Tests: a creator marked non-admin gets no mail (and the suppression claim is untouched), a current admin still does, and the email body for a credential-bearing destination contains the origin but not the path/query sentinel.

## Deferred and follow-up

Carried forward from this round, unchanged in substance:

- **Operation-level OAuth scope matrix** (finding 1's remediation beyond what shipped): per-capability scopes across REST/GraphQL/MCP, plus authorization-decision audit carrying client id, subject, audience, and normalized scopes. Board item, not a remediation-round change.
- **Digest-pinning inventory** (finding 5's residual, seventh report): tenant-version Valkey, `apk add age`, and the wider Dockerfile/kpack/barman/CNPG inventory from ADR061 #1. The answer is ADR060 §D7's internally-built signed toolchain images, not more one-off pins.
- **onbex.co PSL submission** (finding 3, ninth report): operator action, `.pm/w1/050.md`.

The scan's open questions (legacy bare-named App CR inventory, kpack-generated pod shape) are unchanged repeats of ADR055 F2/F3 and ADR061 #2/#6 — both need a live cluster the offline scan and this remediation pass do not have.
