# CLI sweep 664 — IP allow list validation and enforcement (live, 2026-10-19)

Disposable free web service `qa664-web` (`srv-dbbbg5avfpec73dm0jog`). Deleted and verified absent (REST 404).

Via the pinned CLI (`services update --ip-allow-list …`):

| Probe | Result |
| --- | --- |
| `garbage` | client `invalid --ip-allow-list "garbage": must start with cidr=` |
| `cidr=999.1.1.1/33,description=x` | client `invalid CIDR address: 999.1.1.1/33` |
| `cidr=10.0.0.0/8,description=qa664` | 200; readback `"ipAllowList":[{"cidrBlock":"10.0.0.0/8","description":"qa664"}]` |

Enforcement verified end-to-end: with only `10.0.0.0/8` allowed, the public URL immediately answered `403 Forbidden` for the (outside) QA client. No bug filed.
