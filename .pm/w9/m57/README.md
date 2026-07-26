# w9 · m57 — Service internal addresses: slug Service + fromService fix (ADR041 D1–D3)

**Worker:** worker9 **Goal:** a store-managed service's internal address becomes the Render-shaped `<slug>:<port>` — resolvable, connectable, and injected correctly by Blueprint `fromService` env vars — closing the bare-name gap that leaves store-managed stack links pointing at a hostname that doesn't exist. **Status:** todo

## Tasks (in order)

| id   | title                                                                     | est | depends_on |
| ---- | ------------------------------------------------------------------------- | --- | ---------- |
| t001 | Live-capture Render's address surfaces                                     | 45m | —          |
| t002 | Repro the store-managed fromService bare-name gap                          | 45m | —          |
| t003 | Operator: slug-named ClusterIP Service for addressable Apps (D2)           | 60m | —          |
| t004 | Backend: fromService host/hostport resolve to the sibling's slug (D3)      | 60m | t002, t003 |
| t005 | Private-service status.URL → `http://<slug>:<port>` + ADR006 claim fix     | 30m | t003       |
| t006 | Render parity: sweep every surface the address values changed on           | 30m | t004, t005 |
| t007 | Simplify: `/simplify` over the changed code                                | 20m | t006       |
| t008 | Test coverage: slug Service, fromService resolution, status.URL            | 45m | t006       |
| t009 | Closeout                                                                   | 15m | t007, t008 |

## Definition of done

On a live cluster (CAPD mock or dev), a two-service store-managed stack deployed through bex-api (`bex.yml` with a `fromService` host/hostport link) produces: (1) a slug-named ClusterIP Service per addressable App (web + private), App-owned, coinciding with the CR-named Service for legacy slug-less CRs; (2) injected `fromService` env values of the form `<slug>` / `<slug>:<port>` that **resolve and connect** from the sibling pod (same workspace); (3) a private service's `status.URL` reading `http://<slug>:<port>`; and (4) `make verify-tenant-isolation` still passing (a second Service over the same pods changes no reachability). Render's actual address surfaces are captured in `docs/render-artifacts/service-addresses.md` with the repro evidence appended to ADR041 gap 1.

## Source + Goal linkage

- **Source:** [docs/ADR041-service-addresses.md](../../../docs/ADR041-service-addresses.md) (Proposed 2026-07-25), decomposed on user request "achieve address parity with render.com" (2026-07-25). Gap 1 (fromService bare-name literals vs `<tenant>-<name>` CR naming, `core/base.go:728` vs `apps/deploy.go:1426`) is an apparent live correctness bug, not just parity drift.
- **Goal linkage:** Render parity (ADR018's evidence-ledger mission; ADR006 "Render-compatible one core"). The private network address is a core Render primitive every multi-service app depends on.
- **Expected outcome:** internal addresses are Render-shaped and — for store-managed stacks — actually work; ADR006's stale bare-name claim corrected; ADR041 moves toward Accepted.
- **Why now:** the bare-name gap likely breaks every store-managed `fromService` link in prod today (silent: env var injected, connection fails at runtime); the slug mechanism is also the gate for m58's Connect-style surface work.
- **Render parity closing task included:** t004/t005 change REST-visible values (`serviceDetails.url`) and Blueprint semantics — user/tenant-facing surfaces.

## Out of scope

- The Connect-style internal-address **surface** (REST/GraphQL/MCP field + dashboard UX) — that is m58, gated on t001's capture.
- Datastore internal hosts (Postgres `dpg-…`, Key Value) — ADR009/ADR021 own those.
- Per-instance DNS records, multi-port addressing, region axis — ADR041 rejected/conscious-divergence list.
