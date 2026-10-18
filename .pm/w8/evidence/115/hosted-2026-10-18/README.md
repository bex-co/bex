# 115 hosted acceptance — 2026-10-18

[Results](results.json). Production `api.bex.co`, QA workspace bex-canary. Authenticated with a temporary QA API key (client_credentials), revoked after the run. The device login needs the QA password, so it was not used.

- **Description:** live MCP `tools/list` returns 189 tools. `update_service.idleTTLSeconds` now reads: _omit it to keep the current value, 0 selects the platform default window of 15 minutes, a positive value overrides it_. Activity counts "WebSocket frames it received or sent".
- **Semantics:** an owned Free web service, `qa-lx88637d-115-bex`, was created with the native bex CLI v0.3.3 from `traefik/whoami:v1.10.3`.
  - GraphQL `setIdleTimeout` stores 60.
  - MCP `update_service` with `dryRun: true` and the field omitted previews `"idleTTLSeconds":60`.
  - With `"idleTTLSeconds": 0` it previews `0`.
  - The stored value is still 60 afterwards.
- **Cleanup:** the fixture was deleted through the bex CLI (`deleted: true`). No other resource was touched.

The change is text only. The incoming-only WebSocket journey's behavior was already verified live in [sweep372](../README.md) and is unchanged.
