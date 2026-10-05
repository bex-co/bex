# w5 · m122 — Tenant logs: classify lines by source, not by message text

**Worker:** worker5 **Goal:** Platform, kubelet and CNPG-internal lines never reach tenant logs in any pipeline, because classification uses where a line came from, and one fixture set is shared by the shipper and the Go tests. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Close w8/050's remaining gaps | 45m | — |
| t002 | One fixture file for the shipper test and the Go tests | 45m | t001 |
| t003 | Read tenant pod logs from files, as build logs already do | 1h30m | t002 |
| t004 | CNPG: keep Postgres records, drop other JSON | 30m | t002 |
| t005 | Render parity | 15m | t003, t004 |
| t006 | Simplify | 15m | t005 |
| t007 | Test coverage | 30m | t005, t006 |
| t008 | Closeout | 10m | t007 |

## Definition of done

- Live and history reads for apps, Postgres and Key Value show no kubelet "logs gone" lines and no CNPG `net/http` TLS-handshake lines.
- Message-text kubelet filters are gone (source-based classification); any that remain have a recorded reason and identical coverage in every pipeline, enforced by the shared fixtures.
- `scripts/test_log_shipper.py` and the Go log tests read the same fixture file.

## Source + Goal linkage

- **Source:** Last-24h code review, 2026-10-05, of w8/050 (`9ccb92920`) and w8/047 (`c41d1aa91`). Both are deny-lists that grow one QA finding at a time, and the per-pipeline lists have already drifted: Postgres history lacks one kubelet drop, Key Value history lacks both, and Go direct reads lack the CNPG drop.
- **Goal linkage:** ADR010 observability; ADR043 tenant isolation (no platform internals in tenant views).
- **Expected outcome:** Tenant logs contain only tenant output.
- **Why now:** Two deny-list fixes in one day, and the lists already disagree between pipelines.
- **Render parity included:** tenant log contents change on every surface.
- **Boundary:** the shipper config is platform GitOps (`deploy/gitops/base/log-shipper.yaml`); validate changes with the GitOps render/validate scripts.
