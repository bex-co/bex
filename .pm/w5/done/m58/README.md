# w5 · m58 — Metrics Network card Host/Path filters

**Worker:** worker5 **Goal:** close the last parity drift w5/m42 recorded on the service Metrics page: the Network Metrics card gains Render's Host and Path filters, backed by a request-metrics read that accepts `host`/`path` filters (today it rejects them — w3/m12) and by label discovery that offers real values, with store-gated honest states instead of silent ignoring. **Status:** done

## Tasks (in order)

| id   | title                                                                                     | est | depends_on | status        |
| ---- | ----------------------------------------------------------------------------------------- | --- | ---------- | ------------- |
| t001 | Design probe: pin the data source per Network-card section for Host and Path filters       | 45m | —          | — **DONE**    |
| t002 | Backend: accept `host`/`path` filters on the request-metrics read (one core verb)          | 60m | t001       | — **DONE**    |
| t003 | Backend: host/path label discovery for the metrics page (reuse the Loki label values read) | 45m | t002       | — **DONE**    |
| t004 | Surface the filters across REST / GraphQL / MCP (same fields, same reject/503 semantics)   | 45m | t002       | — **DONE**    |
| t005 | UI: Host + Path filter controls on the Network card, store-gated honest states, en/zh      | 60m | t003, t004 | — **DONE**    |
| t006 | Live dev-5 proof + update `docs/render-artifacts/metrics-page.md`'s drift list             | 30m | t005       | — **DONE\***  |
| t007 | Render parity: filter fields/semantics consistent across REST/GraphQL/MCP/UI vs Render     | 30m | t006       | — **DONE**    |
| t008 | Simplify (`/simplify` over the milestone's diff)                                           | 30m | t007       | — **DONE**    |
| t009 | Test coverage: filtered reads, discovery, reject/store-unavailable paths, UI states        | 45m | t007       | — **DONE**    |
| t010 | Closeout                                                                                   | 15m | t009       | — **DONE**    |

**\* t006 live-walk deferral (honest):** the drift record + parity verdicts were updated (`docs/render-artifacts/metrics-page.md`, ADR018:150 corrected), but the **live browser walk was infrastructure-blocked in-session** — `dev-5` could not be raised (the shared kind cluster is missing the CNPG `postgresql.cnpg.io/v1` CRDs the stack needs, and it lacks Loki, which the filtered-series path requires). Per the task's own fallback, the live filtered-series + store-unavailable walk is deferred to the deployed dashboard post-ship (prod carries real Prometheus + Loki); tracked as open note `028`. Implementation is fully verified by the CI-enforced gates (backend `go test ./...` 40 pkgs + golangci-lint; dashboard typecheck + lint + 1,691 tests). Precedent for closing DONE with a recorded live-proof block: `w5/done/m47`.

## Definition of done

On a live bex dashboard (dev-5 or prod), `/services/<id>/metrics` shows Host and Path filters on the Network Metrics card, populated by label discovery; selecting a value visibly changes the request/status-code/response-time series to the filtered subset. With no Loki store configured, any Loki-sourced filter surfaces an explicit store-unavailable state (the Logs-tab 503 pattern) — it is never silently ignored. REST, GraphQL, and MCP accept the same `host`/`path` filters with identical semantics and error shapes, and reject unsupported combinations consistently. `docs/render-artifacts/metrics-page.md` no longer lists Host/Path filters as accepted drift. `cd lego/backend && go test ./...` and `cd dashboard && yarn typecheck && yarn lint && yarn test` pass.

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w5` 2026-07-30 (proposal 1), re-surfacing the accepted-drift list in `.pm/w5/done/m42/README.md` ("Host/Path network filters — bex's metrics API rejects those filters — w3/m12 — and discovery offers no values"). Sibling of `w5/m56`, which closed the other two m42 drifts (percentile "All" overlay, Custom/30d ranges) with the same backend-read-plus-UI shape.
- **Goal linkage:** Render parity (`docs/ADR018-render-parity.md`) — observability surface completeness on the service Metrics page; the "open-source Render alternative" pillar (`docs/ADR008-vision.md`).
- **Expected outcome:** the metrics page's last recorded parity drift is closed; per-host and per-path traffic investigation works from the dashboard, and the same filters are available to agents over MCP and to REST/GraphQL clients.
- **Why now:** it is the only ungated, concretely-recorded w5 gap left after m50–m57 drained the settings/static walks; m56 just built the multi-quantile read this extends, so the metrics core (`lego/backend/internal/metrics/`) context is fresh.
- **Render parity:** included (t007) — the milestone changes REST, GraphQL, MCP, and the dashboard UI.
- **Known design risk (t001 settles it):** Traefik's Prometheus series carry no path label, so Host and Path likely need different sources — Host can ride the existing per-router App attribution (hosts map to Ingress routers), while Path almost certainly needs the Loki request-log store, which already carries `host`/`path` labels for the w5/m23 log filters. That implies the Path filter is store-gated (`BEX_LOKI_URL`), mirroring the Logs tab's honest 503 behavior.
