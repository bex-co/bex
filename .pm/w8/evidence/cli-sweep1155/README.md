# CLI sweep 1155 — metrics are flat `/v1/metrics/{kind}` queries (live, 2026-10-20)

Three disposable free web services created and deleted (204, verified 404). No new bug filed.

- Nested guesses (`/services/{id}/metrics`, `/services/{id}/metrics/cpu`, `/metrics?serviceId=…`) → 404 or a 400 naming the unsupported parameter. The declared surface is flat: `/v1/metrics/cpu|memory|…` with a `resource` parameter (the `service` alias also works).
- A fresh service returns 200 `[]` (no series yet); an unknown resource → 404.
- `/v1/metrics/bandwidth-sources` → 501 with an explanatory message pointing at `/v1/metrics/bandwidth` and GraphQL `monthToDateBandwidth`: a documented ADR018 divergence expressed as a coded refusal, never a bare 404.
