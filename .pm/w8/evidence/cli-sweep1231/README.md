# CLI sweep 1231 — HTTP allow-list `0.0.0.0/0` admits both families (live, 2026-10-20)

Blast-radius control for w8/140. One free web service (`qa-20261020-e4e1de-v6`, `srv-dbbgdc99a20c7385rag0`) created in `bex-canary` with `bex services create … --ip-allow-list cidr=0.0.0.0/0,description=everyone`, deleted with `bex services delete --confirm` (GET 404 verified). No new bug filed.

- Readback `ipAllowList: [{"cidrBlock":"0.0.0.0/0","description":"everyone"}]`. `curl -4` → 200 and `curl -6` → 200. `*.onbex.co` publishes no AAAA (only `A 49.12.20.236`), so HTTP services have no IPv6 path and are unaffected. The datastore wildcards do publish AAAA.
