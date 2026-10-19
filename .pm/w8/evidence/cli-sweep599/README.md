# CLI sweep 599 — `keyvalues update` dialects: policy aliases, persistence, IP list, rename

**Verdict: no bug filed.** Same isolated QA session, `bex-canary`, CLI rebuilt at `1a49cb806`, 2026-10-19. Owned fixture `qa599-kv` (`red-dbb9gmkgnrlc73bhss4g`, free), deleted at close (get → `No Key Value with ID …`, list empty of `qa599`). Create responses are wrapped as `{"data": {…}}`; the first matrix run targeted an empty id because of a harness parse miss — rerun against the real id, so the `400 invalid query parameter "name"` rows in that first pass were self-inflicted, not product behavior.

- **Create-side enums are client-side:** `--memory-policy banana` and `--persistence-mode banana` fail in cobra with the full value list, exit 1, nothing created.
- **Aliases resolve to raw policies:** `cache` → `allkeys_lru`, `queue` → `noeviction`; a raw `volatile_ttl` is stored verbatim. Each update read back exactly via `keyvalues get`.
- **Persistence modes:** `off` and `snapshot` both accepted and read back; invalid values never reach the server (client enum).
- **IP allow list:** replace with `cidr=10.0.0.0/8,description=qa599` stored as `{cidrBlock, description}`; `--clear-ip-allow-list` emptied it; passing both flags fails client-side (`mutually exclusive`); a malformed CIDR fails client-side with `invalid CIDR address`.
- **Rename uses the resource-name grammar** (unlike service display labels, sweep 583): `QA599_Bad!` → `400 (RESOURCE_NAME_INVALID)` with the shared wording. A valid rename takes effect for name resolution immediately; the old name then fails with `No Key Value named 'qa599-kv' …` (the resolver's hint still says `render workspace set` — upstream wording, documented residual).
- **Update guards:** no flags → `at least one field must be provided for update`; synthetic id → `No Key Value with ID 'red-000…'`, exit 1. Updates were accepted while the instance was still `creating`.

## Unrun

- Whether a policy/persistence change restarts a running instance and preserves data (needs an `available` instance plus a connection; sweep 570 covered the flight, not mid-life changes).
- Plan change on update (paid — never submitted by this loop).
