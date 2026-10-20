# CLI sweep 949 — top-level metrics kinds inventory (live, 2026-10-20)

Read-only probes on the baselines (read only). No new bug filed.

- `http-requests`, `bandwidth`, `instance-count` on `hello-go` → 200 with real series (unit `count`); `http-latency`, `cpu-limit`, `memory-target` → 200 `[]` for this window (honest empties); `active-connections` on the baseline Postgres → 200; an invented kind → 404.
- The bandwidth series labels `degraded_sources: "direct"` — the degraded measurement path is declared in the data itself rather than presented as first-class success (the w6/210 degraded-source contract).
