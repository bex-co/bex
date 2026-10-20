# CLI sweep 950 — metrics filter values + stream config read (live, 2026-10-20)

Read-only probes (baselines read only). No new bug filed.

- `GET /metrics/filters/http?resource={hello-go}` → 200 with observed filter values (`statusCode: ["200"]`, `host: []`); `/filters/path` → 200 `[]`; `/filters/application` → 200.
- `GET /metrics-stream/{owner}` → 404 `not found`: no OTLP metrics stream is configured for this workspace, and the read answers not-found rather than fabricating a default config (the configured case is workspace state this loop does not create, since PUT would mutate shared workspace config).
