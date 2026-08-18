# w1 · m69 — Blueprint auto-sync tenant binding: close the identity-less apply pipeline (round-15 scan)

**Worker:** worker1 **Goal:** every resource the git-push Blueprint auto-sync creates or patches is attributed to the blueprint's workspace — tenant-labeled, tenant-namespaced, store-managed, billed, and tenant-scoped — with no identity-less apply path left anywhere. **Status:** done

## Tasks (in order)

| id   | title                                                                                    | est | depends_on |
| ---- | ---------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | core: acting-tenant context consumed ahead of the identity check — **DONE**               | 45m | —          |
| t002 | webhook/blueprint: bind runSync to the row's tenant + fail closed on unresolved tenant — **DONE** | 45m | t001 |
| t003 | Simplify — `/simplify` over the code this milestone changed — **DONE**                    | 20m | t002       |
| t004 | Test coverage — auto-sync attribution + scoped matching + fail-closed refusal — **DONE**  | 45m | t002       |
| t005 | Closeout — **DONE**                                                                      | 10m | t004       |

## Definition of done

- A git-push-triggered Blueprint auto-sync (workspace **developer** role, `autoSync` default-on) creates App/Database/KeyValue CRs carrying `bex.co/tenant: <tenantID>` in `TenantNamespace(tenantID)`, goes through `provisionAppIdentity` (store row, `srv-` id, managed-by/workspace labels, slug), and resolves registry-credential names against the blueprint's workspace — never the bootstrap `default` workspace and never the shared `BEX_API_NAMESPACE`.
- Display-name uniqueness on the sync path is tenant-scoped (`scoped=true`): a same-name CR owned by nobody/another tenant in the shared namespace is never matched or merge-patched, and `fromDatabase` never binds a foreign database's connection Secret.
- When the acting tenant cannot be resolved on the sync path, the apply **refuses** (fail closed) instead of proceeding identity-less.
- `TestBlueprintAutoSyncPreservesTenantContext` asserts CR labels + namespace (not just store rows), and new regression tests fail on the pre-fix code; the backend suite is green.

## Source + Goal linkage

- **Source:** round-15 whole-project security review (Claude Code `/security-review the entire project`, 2026-08-18, HEAD `718a92e6`): five parallel finder passes + per-finding false-positive verification produced one HIGH confirmed at 9/10. The git-push webhook's Blueprint auto-sync goroutine (`lego/backend/internal/apps/webhook.go:341-347`) runs the whole apply pipeline under `core.WithWorkspace(context.Background(), tenantID)` — workspace-named but **identity-less** — so `resolveWorkspaceUncached` (`core/base.go:539-545`) yields an empty tenant and creates land unlabeled in the shared namespace (no store row ⇒ invisible to lists/purge/quota/billing; only the legacy shared quota bounds them), display-name matchers run unscoped and can merge-patch legacy shared-ns CRs (incl. binding a foreign database's `-app` Secret), and registry creds resolve against `default`. The w9/001 fix covered only the store-row half (`resolveTenantID`); its regression test never asserted CR labels/namespace.
- **Goal linkage:** ADR043 (workspace = namespace tenant isolation) integrity, ADR023 usage-metering/billing attribution, and the ADR022/ADR028 security lineage — this is the fifteenth audit round and the first finding that breaks ADR043 attribution from a low-privilege (`developer`, `can_create`) role via an otherwise-correct HMAC-verified webhook.
- **Expected outcome:** the auto-sync path becomes tenant-attributed end to end; the exploit (free unattributed attacker-imaged workloads outside quota/billing/isolation, conditional cross-tenant patch + secret binding on legacy shared-ns clusters) is closed at root cause; a regression test pins every seam the w9/001 test missed.
- **Why now:** live, developer-role-reachable tenant-isolation + billing break on every push to an autoSync blueprint repo; the conditional cross-tenant half depends only on legacy shared-namespace datastores existing, which the datastore-namespace-cutover runbook documents as a supported live state.
- **Render parity omitted:** internal webhook/sync pipeline fix — no REST/GraphQL/MCP/UI contract changes (no request/response shape, field, or error dialect is touched; the webhook's HTTP contract is unchanged).

## Notes

- Repo rule: work stays uncommitted until the user runs `/ship`.
- The identity-less arm of `AuthorizeLabeled`/`GetApp` itself is pre-existing intentional design for the HMAC redeploy path (which only patches repo-matched Apps) — this milestone binds the Blueprint sync to a real tenant rather than narrowing that arm, unless implementation reveals the narrowing is required.

## Outcome (2026-08-18)

**Done, same session.** `core.WithActingTenant`/`ActingTenantFrom` (`core/workspace.go`) gives platform background callers a server-derived tenant that `resolveWorkspaceUncached` honors after (only after) the identity check — identity stays authoritative for request-shaped callers, and an acting/named disagreement fails closed (`ErrAuthzUnavailable`). `triggerBlueprintSync` binds it from the `GetBlueprintByRepo` row; `runSync` opens with the fail-closed guard (`ErrBlueprintSyncWorkspaceUnresolved`, store-on only). With the binding, the whole apply pipeline takes the tenant branches: `createNewApp` stamps `LabelTenant` + `CRName` + the ADR043 tenant namespace and runs `provisionAppIdentity`; `unique{Database,KeyValue}ByDisplayName` get `scoped=true` (foreign/unlabeled shared-ns CRs can no longer be matched or merge-patched); `AuthorizeLabeled`'s identity-less no-gate arm no longer applies on this path (unlabeled ⇒ nobody's ⇒ `ErrForbidden`); registry creds resolve against the blueprint's workspace, not bootstrap `default`. The HMAC redeploy path is untouched (no acting tenant is ever set on it).

Evidence: `TestActingTenantResolvesWithoutIdentity` / `TestActingTenantYieldsToIdentity` (core), `TestBlueprintAutoSyncPreservesTenantContext` (rewritten — now asserts CR labels/namespace + the store-row half it used to only test), `TestBlueprintAutoSyncScopedMatchingIgnoresForeignResources`, `TestRunSyncFailsClosedWithoutResolvableTenant`; the binding + guard regressions verified **red on the pre-fix code** before restoring. Full backend suite + backend golangci-lint green.

Simplify pass (3 parallel reviewers): applied test-literal dedup, comment de-duplication (the round-15 rationale now lives once, on the sentinel), and the conflict-collapse note on the guard. Skipped as not worth it: exporting a memo-threading helper to save one `Workspace.Tenant` round trip per manual sync (cold path), and routing the scoped-matching test through `triggerBlueprintSync` instead of `runSync` (it deliberately pins `runSync`'s own contract).
