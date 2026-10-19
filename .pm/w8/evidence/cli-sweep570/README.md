# CLI sweep 570 — Key Value full CLI flight (free slot)

**Verdict: no bug filed.** Same isolated QA session (`puncsky@gmail.com`), `bex-canary`, HEAD-built CLI at `77ec448ae`, production pin `d98d07f62`, 2026-10-19T19:21–19:25Z. The workspace's single free Key Value slot was open (another loop's store had been removed). One refusal probe with a valid CIDR and omitted description turned out to be a legal create, so `qa570-refuse` (`red-dbb6rett4o5c73dqgd3g`) was adopted as the owned fixture, flown end to end, then deleted (CLI get named-not-found, REST 404, list `[]`).

- **Create refusals, layer-correct:** bogus plan → boundary `invalid request body at /plan`; bogus region / memory-policy / persistence-mode → pinned-client enum errors before any call; malformed `--ip-allow-list` → client grammar errors naming the expected `cidr=` form and the bad CIDR. Second free create → `400 (WORKSPACE_RESOURCE_LIMIT): workspace is limited to 1 key-value store; delete an existing key-value store to create another`, nothing created.
- **Read shapes complete:** REST get carries every pinned-`keyValue`-required key (`id/createdAt/updatedAt/status/region/plan/name/owner/options/ipAllowList/version/dashboardUrl`) — the w8/129 lesson re-checked on the sibling kind; no `suspenders` is required here. The CLI projection flattens `options` to `maxmemoryPolicy`/`persistenceMode`. Defaults: `allkeys_lru` + `journal_snapshot`, version `8`. Connection strings were never printed (key names only).
- **Updates exact:** friendly aliases resolve on read-back (`queue`→`noeviction`, `cache`→`allkeys_lru`); persistence `off`→`journal_snapshot` round-trips through visible `config_restart`; `--ip-allow-list` replaces wholesale (prior entry gone), set+clear together → client mutual-exclusion error, clear → `[]`; bogus plan → boundary 400; no-flag update → client "at least one field must be provided".
- **Suspend/resume:** suspend settles `suspended` (REST `status`/`suspended` agree); resume-on-active and double-suspend are idempotent 200s; a policy update **while suspended** is accepted and held — after resume the store came back `available` with `maxmemoryPolicy: noeviction` applied through `config_restart`, not silently dropped.
- **Selectors:** get by name resolves; synthetic `red-000…` and a service id under `keyvalues suspend` → the client's named "No Key Value named … in workspace …" resolution error (upstream wording, workspace-correct).

## Unrun

- `kv-cli` interactive session (PING/SET round trip) — the session verb is TTY-oriented; data-plane proof rests on prior loop eras (sweep 481 persistence toggles; ADR021).
- Paid plan changes and external-access enablement (paid/billing surface, out of QA scope).
