# CLI sweep 1106 — MCP creates are per-type (live, 2026-10-20)

Read-only MCP catalog probes. No fixture created. No new bug filed.

- There is no generic `create_service` MCP tool: creation is per-type (`create_web_service`, `create_static_site`, `create_cron_job`, `create_key_value`, `create_postgres`, …), while reads/updates/deletes use the generic `get_service`/`update_service`/`delete_service` names (sweeps 1104/1105). The catalog shape mirrors Render's MCP tool taxonomy.
