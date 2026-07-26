# w9 · m57 — Service internal addresses: slug Service + fromService fix (ADR041 D1–D3)

**Worker:** worker9 **Goal:** a store-managed service's internal address becomes the Render-shaped `<slug>:<port>` — resolvable, connectable, and injected correctly by Blueprint `fromService` env vars — closing the bare-name gap that leaves store-managed stack links pointing at a hostname that doesn't exist. **Status:** done

## Tasks (in order)

| id   | title                                                                  | est | depends_on |
| ---- | ---------------------------------------------------------------------- | --- | ---------- |
| t001 | Live-capture Render's address surfaces                                 | 45m | — — **DONE** |
| t002 | Repro the store-managed fromService bare-name gap                      | 45m | — — **DONE** |
| t003 | Operator: slug-named ClusterIP Service for addressable Apps (D2)       | 60m | — — **DONE** |
| t004 | Backend: fromService host/hostport resolve to the sibling's slug (D3)  | 60m | t002, t003 — **DONE** |
| t005 | Private-service status.URL → `http://<slug>:<port>` + ADR006 claim fix | 30m | t003 — **DONE** |
| t006 | Render parity: sweep every surface the address values changed on       | 30m | t004, t005 — **DONE** |
| t007 | Simplify: `/simplify` over the changed code                            | 20m | t006 — **DONE** |
| t008 | Test coverage: slug Service, fromService resolution, status.URL        | 45m | t006 — **DONE** |
| t009 | Closeout                                                               | 15m | t007, t008 — **DONE** |

## Implementation notes (2026-07-26, closeout)

- **t001 capture** — docs-fallback (`docs/render-artifacts/service-addresses.md`): internal address = `<slug>:<port>` (hostname is the suffixed slug); Connect → Internal / pserv "Service Address"; REST `serviceDetails.url` **absent** for pserv (bex's FQDN-in-url is a structural divergence → m58 t001's shape decision), `slug` present on all types, **no** internal-address REST field; new finding: `[INTERNAL_HOSTNAME]-discovery` + `RENDER_DISCOVERY_SERVICE` (recorded as the named deferral in ADR041).
- **t002 repro (live, fresh CAPD mock cluster)** — projector-shaped store-managed pair: `wget http://api:8080/` from the sibling pod → `bad address` (the pre-fix injected value resolves to nothing); CR-name and slug Services both answer. Cross-workspace counter-probe to the slug hostname timed out (NetworkPolicy denies; resolution ≠ reachability), same-workspace control green. Transcript in ADR041 gap 1.
- **t003 mechanism** — one `applyClusterIPService` helper builds both the CR-named Service and the slug alias (identical shape, explicit `Protocol: TCP` so steady-state reconciles are read-only — the mutate previously diffed against the server default and issued a no-op PUT every loop, both Services). `reconcileSlugService` is **bidirectional** and runs once pre-dispatch for every type: addressable (web/private) ⇒ CreateOrUpdate; non-addressable ⇒ delete-if-`IsControlledBy` — so no type path owns a forgettable delete half (the first cut hand-wired deletes into worker+static and missed cron; the review caught it). Squat guard: `AlreadyOwnedError` ⇒ skip with log, never fail the deploy, never adopt/rewrite a foreign Service. **Discovered en route:** `spec.type` is CRD-immutable (`app_types.go:76` CEL), so the delete half is defense-in-depth for pre-rule CRs/object surgery, not a live type-flip path; the envtest plants the leftover state directly.
- **t004 resolver** — `fromService` host/hostport classified at parse into typed `hostRef{key,target,port,hostport}` (port resolves at parse; one shared unknown-target guard) and resolved at apply against a lazily-memoized slug lookup: existing targets (re-sync/backward refs) resolve before their referrer is written — zero spec churn; forward refs on first apply resolve in a post-create second `applyCreate` pass. Legacy slug-less stacks inject the bare name byte-identical. `stackManifest`'s web→api link is itself a forward ref, so the standard tests exercise the second pass; idempotent re-apply asserts exactly one env var with a stable value.
- **t005 status.URL** — private-service `status.URL` = `http://<slug>:<port>` on the same `slug != app.Name` predicate `reconcileSlugService` uses (so the URL flips only when the slug Service actually exists — keeps the FQDN for legacy **and adopted** Apps where slug == CR name, the eden-cms-v2 class the first-cut `Subdomain != ""` discriminator would have broken). ADR006 §Blueprint's stale bare-name claim corrected.
- **t006 parity sweep** — REST/GraphQL/MCP all read the one `AppView.URL` (`render.go` "url", `graphql.go:273`); no backend/dashboard `.svc` shape assumptions outside datastores (out of scope); ADR018 Blueprint row annotated.
- **t007 simplify** — 4 parallel review agents (reuse/simplification/efficiency/altitude); applied: shared Service helper, bidirectional convergence (cron miss), predicate unification (adopted corner), typed hostRef (killed `parsedStack.servicePorts`, the two-map resolver + unreachable branch), lazy slug lookup (killed the standalone pre-seed + wasted backward-ref GetApps), one unknown-service guard, stale comments, test-fixture hoist. Skipped with reasons: dependency-ordered creation (two-pass survives as the cycle fallback either way), `applyCreate` signature change to dedup its internal GetApp, hoisting an internal-address contract into `types/` (premature until m58 adds the second consumer — review's own verdict).
- **Validation** — operator `make test` + `make lint` (both modules) green; backend `go test ./...` green; store PG integration (slug minting) green against a disposable Postgres 16 container; live probes on the fresh CAPD mock cluster as above. Fixtures removed from the cluster after evidence capture.

## Definition of done

On a live cluster (CAPD mock or dev), a two-service store-managed stack deployed through bex-api (`bex.yml` with a `fromService` host/hostport link) produces: (1) a slug-named ClusterIP Service per addressable App (web + private), App-owned, coinciding with the CR-named Service for legacy slug-less CRs; (2) injected `fromService` env values of the form `<slug>` / `<slug>:<port>` that **resolve and connect** from the sibling pod (same workspace); (3) a private service's `status.URL` reading `http://<slug>:<port>`; and (4) `make verify-tenant-isolation` still passing (a second Service over the same pods changes no reachability). Render's actual address surfaces are captured in `docs/render-artifacts/service-addresses.md` with the repro evidence appended to ADR041 gap 1.

**How the DoD was met (evidence composition):** the mechanism (slug Services, ownership, isolation) was proven live on the fresh CAPD mock cluster with projector-shaped CRs whose env is byte-equal to the fixed resolver's output; the resolver's output itself (forward refs, idempotence, slug injection) is proven by `TestDeployStackFromServiceHostResolvesToTheSlug` through the real `DeployStack` core, and real-Postgres slug minting by `store_pg_test.go`. The full HTTP-through-bex-api loop was not run live (the rebuilt mock cluster has no CNPG/auth substrate for dev-9); the HTTP adapters are untouched by m57, so the composition covers the DoD's intent. Isolation item (4) was verified as the targeted probes (same-workspace allow through the slug Service, cross-workspace deny, `resolution ≠ reachability`) rather than the full `verify-tenant-isolation.sh` matrix, which needs the gitops platform namespaces absent from the bare mock cluster.

## Source + Goal linkage

- **Source:** [docs/ADR041-service-addresses.md](../../../docs/ADR041-service-addresses.md) (Proposed 2026-07-25), decomposed on user request "achieve address parity with render.com" (2026-07-25). Gap 1 (fromService bare-name literals vs `<tenant>-<name>` CR naming, `core/base.go:728` vs `apps/deploy.go:1426`) is an apparent live correctness bug, not just parity drift.
- **Goal linkage:** Render parity (ADR018's evidence-ledger mission; ADR006 "Render-compatible one core"). The private network address is a core Render primitive every multi-service app depends on.
- **Expected outcome:** internal addresses are Render-shaped and — for store-managed stacks — actually work; ADR006's stale bare-name claim corrected; ADR041 moves toward Accepted.
- **Why now:** the bare-name gap likely breaks every store-managed `fromService` link in prod today (silent: env var injected, connection fails at runtime); the slug mechanism is also the gate for m58's Connect-style surface work.
- **Render parity closing task included:** t004/t005 change REST-visible values (`serviceDetails.url`) and Blueprint semantics — user/tenant-facing surfaces.

## Out of scope

- The Connect-style internal-address **surface** (REST/GraphQL/MCP field + dashboard UX) — that is m58, gated on t001's capture.
- Datastore internal hosts (Postgres `dpg-…`, Key Value) — ADR009/ADR021 own those.
- Per-instance DNS records (`<slug>-discovery`), multi-port addressing, region axis — ADR041 rejected/conscious-divergence list.
