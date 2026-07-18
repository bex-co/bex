# w7 · m45 — Key Value update parity: honor memory-policy + IP allow-list on the update PATCH

**Worker:** worker7 **Goal:** the unmodified Render CLI's `keyvalues update --memory-policy` / `--ip-allow-list` / `--clear-ip-allow-list` actually mutate the instance (today they are accepted but silently no-op) **Status:** done

## Tasks (in order)

| id   | title                                                                                          | est | depends_on |
| ---- | ---------------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Honor `maxmemoryPolicy` on the Key Value update PATCH — **DONE**                                | 45m | —          |
| t002 | Honor `ipAllowList` (+ clear) on the Key Value update PATCH — **DONE**                          | 45m | —          |
| t003 | Render parity: KeyValue update memory-policy + IP allow-list across REST/GraphQL/MCP/dashboard — **DONE** | 30m | t001, t002 |
| t004 | Simplify the changed Key Value update path — **DONE**                                           | 20m | t003       |
| t005 | Test coverage: update actually mutates memory-policy + allow-list + clear — **DONE**            | 40m | t003       |
| t006 | Closeout — **DONE**                                                                             | 10m | t005       |

## Definition of done

Driving the **unmodified** official Render CLI against bex-api:

- `keyvalues update <kv> --memory-policy queue` changes `maxmemoryPolicy` to `noeviction` on read-back (a real diff, not the current empty-diff no-op);
- `keyvalues update <kv> --ip-allow-list cidr=203.0.113.0/24,description=hq` replaces the allow-list and the entry (CIDR **and** description) returns on the next `get`;
- `keyvalues update <kv> --clear-ip-allow-list` empties the allow-list;
- the same semantics hold across REST, GraphQL, MCP, and the dashboard;
- a test asserts each previously-silent no-op now mutates (returns a non-empty diff and the read-back reflects it).

## Source + Goal linkage

- **Source:** `docs/cli-compatibility-checklist.md` verification pass (this session, 2026-07-18) — the six-agent CLI-compat sweep found `keyvalues update --memory-policy | --ip-allow-list | --clear-ip-allow-list` return `200` with an empty diff and leave the field unchanged; root cause `handleUpdateKeyValue` (`lego/backend/internal/keyvalue/rest.go`) and `KeyValuePatch` (`.../service.go`) decode only `name`/`plan`. (The `/pm` request said "keyvalues **create**", but the sweep verified `create` fully working on every flag — the silent no-op is on **update**; scoped here to the real defect.)
- **Goal linkage:** Render API compatibility ([docs/ADR006-bex-api.md](../../../docs/ADR006-bex-api.md), [docs/ADR018-render-parity.md](../../../docs/ADR018-render-parity.md)) — bex-api must accept the unmodified `render` CLI, and per `DO_NOT_DO.md` the gaps the cli-compatibility-checklist surfaces become work against **bex-api**, never a forked CLI. Directly extends **w7/m5** (Managed Key Value network access controls / ipAllowList parity), whose update path this closes.
- **Expected outcome:** the unmodified Render CLI (and every bex surface) can change a Key Value instance's eviction policy and IP allow-list after creation; no more misleading silent-success.
- **Why now:** a silent no-op is worse than a rejection — the CLI reports success while nothing changes — and it is a live parity defect on the KV surface w7/m5 already owns; cheap to fix while the finding is fresh, before real tenants rely on the update path. **Render-parity task included** — the fix changes a user/tenant-facing surface exposed across REST, GraphQL, MCP, and the dashboard.
