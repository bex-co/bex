# CLI sweep 1065 — CLI services update lands in REST state (live, 2026-10-20)

Two disposable free web services created and deleted (204s; GETs 404). No new bug filed.

- `bex services update {id} --health-check-path /healthz1065 --confirm` exits 0, and the REST GET immediately reads `healthCheckPath: /healthz1065` back. CLI writes and REST reads share one store (an earlier probe in this sweep deleted its fixture before the read-back parse; the redo above is the authoritative pass).
