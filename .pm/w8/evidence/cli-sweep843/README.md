# CLI sweep 843 — CLI `services update` round-trip (live, 2026-10-20)

Disposable free web service created via REST, updated via CLI, deleted (204, then GET 404). No new bug filed.

- `bex services update <id> --name qa843-renamed --health-check-path /hc843 --confirm` → applied; REST GET independently confirms `healthCheckPath: /hc843` and the new display name (CLI write, REST read-back — the two surfaces agree).
- `--plan bogus843` → server 400 `plan must be one of free|starter|standard|pro|pro_plus|pro_max|pro_ultra`, surfaced verbatim by the CLI with a non-zero exit (bogus-enum side only, per the sweep-777 lesson — no real plan change was attempted).
