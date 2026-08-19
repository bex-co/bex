# ADR067: Security review round 12 — qVt3XO disposition

- **Status**: Accepted (2026-08-17)
- **Scan**: codex-security `qVt3XO`, repository revision `9fdb394a` (2026-08-17), 10 findings (1 high, 5 medium, 4 low)
- **Lineage**: twelfth pass in the ADR028 → ADR045 → ADR055 → ADR056 → ADR057 → ADR072 → ADR061 → ADR063 → ADR064 → ADR066 lineage

## Summary

Seven of ten findings are addressed in place with regression tests, one high finding decomposes into the standing migration-gated deferrals **plus four previously untracked gaps that are now fixed**, and the remaining two are re-confirmed standing residuals (PSL submission, digest pinning). No finding was rejected outright.

| # | Finding | Severity | Disposition |
| --- | --- | --- | --- |
| 1 | Tenant-local App names collide across shared registry, storage, execution infra | high | **Split**: data-level identities (OCI repo/Zot, static prefix/purge) = ADR055 F2/F3 re-confirmed (migration-gated); **4 untracked build-namespace object gaps fixed in place** |
| 2 | Static cache metadata grows outside byte budgets | medium | **Fixed** — per-entry charge (key + content-type + fixed overhead) in the one cost model shared by put/evict |
| 3 | Unbounded static route/header rules amplify public requests | medium | **Fixed** — item + field-length caps in CRD schema and surface validators (100 items; 2048/256/4096 chars) |
| 4 | GraphQL alias limits permit hundreds of expensive resolver calls | medium | **Fixed** — total alias budget 500→50 plus a new per-field-name alias cap (10) |
| 5 | Direct project/environment creation creates durable pre-pagination fan-out | medium | **Fixed** — direct creates share the Blueprint grouping quota transactionally; REST list projectId fan-out deduped + capped at 20 |
| 6 | Shared tenant domain permits cross-tenant cookie tossing | medium | **Accepted residual** — onbex.co PSL submission (seventh report), blocked on operator action |
| 7 | API-key quota/inventory read only the first 500 Hydra clients | low | **Fixed** — cursor-following Hydra list (Link rel="next", 20-page bound) |
| 8 | Mutable backup images + runtime `apk add age` in plaintext path | low | **Deferred** — digest-pinning inventory (fifth report), extends to the age stage |
| 9 | Unknown PostgreSQL SNI scans the global route table | low | **Fixed** — precomputed hostname-variant index (O(1) resolve, kv-sni-proxy parity) |
| 10 | Unsigned event header reinterprets a signed webhook body | low | **Fixed** — body-shape binding (push requires `refs/` ref + no `ref_type`) before scope/claim |

## Finding 1 (high) — decomposition

The scan's headline repeats ADR055's root cause ("workspace-local `App.Name` used as a global identity at shared sinks"). Round-level evidence audit at HEAD split it into two very different halves:

**Standing deferrals, re-confirmed unchanged** (7th report of the same family):

- OCI repository `<registry>/<app.Name>:gen-N`, Zot user `app-<name>`, repo ACL, and the finalizer's tag purge are all name-keyed with no tenant qualifier — ADR055 **F3** (data-migration-gated: re-keying invalidates image refs baked into running Deployments).
- Static-site object prefix `<app.Name>/<rev>/` and the purge prefix `<app.Name>/` — ADR055 **F2** (re-keying orphans published sites until redeploy).

Both require the coordinated dual-read → migrate → redeploy rollout ADR055 already scoped. The exploitation prerequisite also stands: bex-api's store path tenant-prefixes CR names (`<ws>-<name>`), so a same-named pair needs a legacy (never-renamed) or hand-applied App CR.

**Four untracked gaps, fixed in place** — the build namespace is shared across workspaces (ADR043) while these objects derive their names from the bare App name, and each path was missing the ownership gate its siblings already had:

1. **`bld-<name>-registry-auth` Secret** (`prepareBuildRegistrySecret`): a bare `CreateOrUpdate` let a same-named App in another workspace silently overwrite this App's external-registry credential and relabel ownership — cross-tenant credential swap into the other tenant's BuildKit pod. Now fails closed via the shared `checkOwnedArtifact` mutate-guard (a foreign-owned object is an error, exactly like `build.EnsureBuild`'s CheckOwner).
2. **`reg-pull-<name>` build-ns mirror** (`copyBuildRegistryCredential`): same silent-overwrite shape, same fix.
3. **Execution NetworkPolicy** `bld-<name>-execution-egress`: the reconcile's `CreateOrUpdate` flapped spec+labels between same-named tenants (the round-5 workspace-scoped PodSelector kept the _grant_ tenant-correct, but each tenant's pre-deploy egress only worked while its own reconcile's version was live), and the finalizer deleted the object by bare name — cross-tenant teardown. Both paths now owner-gated.
4. **Finalizer's build-ns `reg-pull-<name>` deletion** (`revokeAppRegistryCredentials`): bare `deleteAndWait` removed the same-named App's build/pull credential. Now routed through the generalized `deleteOwnedObject` (the codex-security #5 gate, extended).

`deleteOwnedSecret` was generalized into `deleteOwnedObject` for the NetworkPolicy/Secret shapes. Tests: `TestBuildNamespaceSecretsRefuseForeignOwner`, `TestDeleteOwnedObjectLeavesForeignArtifacts` (`lego/operator/internal/controller/registry_auth_test.go`).

Residual within the residual: build/pre-deploy/publish Job names remain name-derived, but their `identity.CheckOwner` fail-closed behavior makes collisions a mutual DoS, not a cross-tenant effect (ADR055's accepted posture); `RevokeCreds(app.Name)` and the Zot repo purge stay name-keyed inside the F3 deferral.

**Update (w2/m75):** `RevokeCredsFor` + the finalizer purge now consume `lego/operator/internal/identity` ([ADR074](ADR074-workspace-scoped-artifact-identity.md)) — a labeled App purges `W/A` (and the legacy location only when tombstoned). The Job-name residual is unchanged.

## Fixed findings

### 2 — static cache metadata accounting

`cache.put` charged only `len(obj.Body)`, so a zero-byte entry was inserted with zero cost and never triggered eviction — a tenant publishing millions of empty files and warming them publicly could grow the shared single-replica server's map/bookkeeping toward its memory limit while both byte budgets read as unexhausted. The entry cost is now `len(body) + len(key) + len(contentType) + entryOverhead (256 B)` — one charge model shared by `put` and `evictOne` so an entry is always released for exactly what it was charged. Test: `TestCacheChargesEntryMetadata`.

### 3 — static edge-rule budgets

Routes/headers had no item or length bounds anywhere (CRD schema or surface validators), while the shared static server linearly scans routes and header rules on every unauthenticated request. Caps (mirrored in `lego/types/v1alpha1/app_types.go` MaxItems/MaxLength — regenerated CRD — and enforced as clean 400s in `validateRoutes`/`validateHeaders` so all three surfaces reject before any CR write): 100 routes, 100 headers, 2048-char paths, 256-char header names, 4096-char header values. Test: `TestStaticEdgeRuleBudgets`.

### 4 — GraphQL alias amplification

`gqlMaxAliases = 500` let one authenticated document run a full-workspace list resolver (projects: all rows + per-project membership enrichment) 500 times while the rate limiter charged one token. The structural gate now bounds aliases at 50 total **and 10 per field name** — the amplification shape is one field aliased repeatedly; diverse aliased reads are not. Fragment-spread resolution means the per-field cap holds through fragments too. Tests: `TestGraphQLPerFieldAliasBudget` (plus the existing budget test, which keys off the constant).

### 5 — grouping quota on direct creates + list fan-out

Only the Blueprint apply loop enforced `BEX_MAX_BLUEPRINT_GROUPINGS`; direct REST/GraphQL/MCP project/environment creates were unbounded durable state, and list paths materialize + enrich every row before transport pagination. `projects.Service` and `environments.Service` now carry `MaxGroupings` (wired from the same `BEX_MAX_BLUEPRINT_GROUPINGS` dep) and enforce it **transactionally** via `RunGroupingTx` (count + insert in one tx) where the store offers the runner — the production `*PGStore` always does — refusing with the same coded `BLUEPRINT_GROUPING_LIMIT` (0 disables, matching the Blueprint path). The environment count is checked against the project's own workspace. The REST `GET /v1/environments` multi-`projectId` form (REST-only; GraphQL and MCP take exactly one) now deduplicates and refuses past 20 distinct ids before the first list runs. Tests: `TestCreateEnforcesGroupingQuota`, `TestCreateEnvironmentEnforcesGroupingQuota`, `TestREST_ListEnvironmentProjectIDFanOutCap`.

### 7 — Hydra client list pagination

`hydraAPIKeys.List` issued one `GET /admin/clients?page_size=500` — not a documented Hydra parameter — and read only the default first page. Past 500 global clients, workspace keys on later pages were invisible to the active-key quota and inventory. The list now follows Hydra's cursor (`Link` header, `rel="next"`), keeping only the path?query of the advertised URL so the loop stays pinned to the admin endpoint, bounded at 20 pages; a Link-free response degrades to the old one-page behavior. Termination is the Link header alone (a short page may still carry a cursor). Test: `TestHydraListFollowsCursorPages`.

### 9 — pg-sni-proxy hostname index

`dbRouter.resolve` iterated the entire route table under the read lock for every parsed ClientHello — an unknown-SNI flood paid O(global databases) work pre-authentication on a 100m-CPU shared proxy (the kv-sni-proxy sibling always used a map). The router now precomputes every public hostname variant (rw, `-pool`, `-ro-<replica>`) into a `hosts` index with the backend address resolved at reconcile time; `resolve` is one map hit plus the (already scoped) allowlist check, with a per-label candidate slice preserving the degenerate fall-through where two databases claim one label (e.g. db `x` with pooler vs a database literally named `x-pool`). Tests: `TestRouterHostnameIndexIsExact` + the existing `TestRouterResolve` semantics.

### 10 — git webhook body-shape binding

The HMAC authenticates the body, but event-type selection read the unsigned `X-GitHub-Event` header **before** the replay claim: a captured signed `delete` payload replayed with the header stripped (the pre-fix Gitea fallback treated "no header" as push) or set to `push` unmarshaled into a plausible `pushEvent` (short ref, empty `after` — not zero-SHA, so not treated as a deletion) and could redeploy while consuming the replay claim the legitimate delete would need. The push branch now requires the mutually exclusive push invariants — a full `refs/…` ref (GitHub and Gitea both send it) and **no** `ref_type` (the field that marks delete/create payloads) — rejecting anything else with 400 before scope resolution or claim; the delete branch symmetrically ignores a `refs/`-prefixed ref. Tests: `TestWebhookRejectsHeaderSwappedDeleteBodyAsPush`, `TestWebhookDeleteHeaderOnPushBodyIsNoOp`.

## Re-confirmed residuals

- **6 (medium, accepted)** — `onbex.co` is not a browser-enforced Public Suffix, so sibling tenant origins can toss `Domain=onbex.co` cookies at each other. Seventh report (= ADR055 F9 → ADR072 1 → ADR061 4 → ADR063 3 → ADR064 → ADR066). The code already detects and warns loudly (`hostingdomain.ErrUnlistedSharedSuffix`); the fix is the PSL submission, blocked on operator action (`.pm/w1/050.md`). Startup stays deliberately non-fail-closed to avoid a self-outage.
- **8 (low, deferred)** — the Key Value backup pipeline runs mutable Valkey/BusyBox/Alpine tags and installs `age` at runtime via `apk add` in a stage that reads the plaintext RDB. Fifth report of the digest-pinning inventory deferral (ADR061 1 → ADR063 12 → ADR066 7 → ADR064), now explicitly extended to cover the age stage's runtime package install; the durable fix is the reviewed first-party backup helper ADR061 already scoped. Snapshot-stage `REDISCLI_AUTH` exposure is bounded by the same supply-chain prerequisite (upstream compromise).

## Deferred-with-evidence register (carried)

| Item | First report | This round |
| --- | --- | --- |
| OCI/Zot + static-prefix tenant identity migration | ADR055 F2/F3 | re-confirmed (finding 1). **Update (w2/m75):** code landed ([ADR074](ADR074-workspace-scoped-artifact-identity.md)); prod cutover [runbook-gated](runbooks/registry-static-identity-migration.md) until phase 4. |
| onbex.co PSL submission | ADR055 F9 | re-confirmed (finding 6, 7th) |
| metrics-server kubelet TLS | ADR072 5 | not re-reported |
| digest-pinning inventory (incl. `apk add age`) | ADR055 F7 | re-confirmed (finding 8, 5th) |
