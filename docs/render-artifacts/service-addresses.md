# Capture — Render's service address surfaces (w9/m57 t001)

**Captured:** 2026-07-26 · **Method:** docs-fallback (render.com/docs/private-network, /docs/private-services, api-docs.render.com/reference/service-fields) — no live Render account was available; anything below not confirmable from public docs is marked open. The design source for ADR041 D4 / w9/m58's field-shape decision.

## 1. Internal address format

> "The private service above has the internal address `elasticsearch-2j3e:9200`." — private-network doc

- Shape: **`<hostname>:<port>`** — bare hostname, no DNS suffix, no namespace segment.
- The hostname is the **slug** (both doc examples — `elasticsearch-2j3e`, `elastic-qeqj` — carry the global-collision suffix; see [duplicate-service-names.md](duplicate-service-names.md)).
- Protocol prefix is caller-supplied: "You might need to specify a service's expected protocol in its internal address string when you connect" — e.g. `http://elastic-qeqj:10000`.
- The `:10000` example is Render's default injected `PORT` for web services (bex default: 3000 — conscious divergence, ADR041 D5).

## 2. Where the dashboard shows it

- Every service's **Connect** menu → **Internal** tab shows the internal address (web + private services, Postgres, Key Value).
- A **private service** additionally displays it as its **Service Address**.
- The **deploy-detail header** shows an internal address ([deploy-detail-page.md](deploy-detail-page.md) recorded this and bex's deliberate scope-out).

## 3. REST shape (api-docs service-fields table)

| Field | web_service | private_service | background_worker | Evidence |
| --- | --- | --- | --- | --- |
| `serviceDetails.url` | ✅ present (public URL) | **❌ absent** | ❌ absent | service-fields table |
| `slug` | ✅ | ✅ | ✅ | "Field is present" for slug on all five types |
| any internal-address field | ✖ none documented | ✖ none | ✖ none | service-fields table |

**Implications for bex:**

- bex putting `http://<crName>.<ns>.svc:<port>` into a **pserv's** `serviceDetails.url` is a structural divergence (Render omits the field entirely), not just a value divergence — m58 t001 decides omit-vs-extend; w9/m57 t005 only fixes the _value_ the operator writes to `status.URL`.
- Render's REST exposes **no** internal-address field at all — consumers derive `<slug>:<port>` from the `slug` field (present on every type) + their known port. bex already returns `slug` on the service object (`apps/service.go:278`, w4/m19). The dashboard-side Connect surface is where Render materializes the derived string.

## 4. Scope + addressability

> "Other Render services are on the same private network if they're deployed in the same region _and_ they belong to the same workspace."

- Addressable types: web services, private services, Render Postgres, Key Value. Background workers/cron jobs can dial out but have no internal address ("do not receive an `onrender.com` subdomain"; no address field on any surface).
- bex analog: workspace scope enforced by NetworkPolicy reachability, no region axis (ADR041 D5).

## 5. Multi-instance discovery (new finding, beyond ADR041's first draft)

> "each has a **discovery hostname** that resolves to _all_ of its active instance IPs. By convention, this hostname has the format `[INTERNAL_HOSTNAME]-discovery` (e.g., `myapp-ne5j-discovery`)" "Each service exposes its discovery hostname to its own environment via the `RENDER_DISCOVERY_SERVICE` environment variable."

- The plain internal hostname is the load-balanced address; `<hostname>-discovery` is the all-instances DNS name.
- bex analog would be a **headless Service** named `<slug>-discovery` (per-pod A records) + a `RENDER_DISCOVERY_SERVICE` env injection — deferred in ADR041 (rejected options, per-instance DNS); recorded here so the deferral names the exact Render behavior it defers.

## 6. Open questions (need a live account; docs don't answer)

- Whether the Connect → Internal tab prints the protocol prefix or the bare `host:port` string (both appear in docs prose).
- Whether `serviceDetails` for a pserv carries a port field the dashboard uses, or the dashboard reads the service's declared port from elsewhere.
