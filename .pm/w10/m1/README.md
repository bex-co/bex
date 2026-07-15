# w10 · m1 — Restore operator reconciliation under namespace-scoped Secret RBAC

**Worker:** worker10 **Goal:** restore stable operator startup and App/KeyValue reconciliation without granting cluster-wide Secret access **Status:** todo (t001-t005 done)

## Tasks (in order)

| id   | title                                                             | est | depends_on |
| ---- | ----------------------------------------------------------------- | --- | ---------- |
| t001 | Reproduce the cache-sync failure and design a namespace-safe watch — **DONE** | 30m | —          |
| t002 | Implement the least-privilege KeyValue Secret watch — **DONE**                | 45m | t001       |
| t003 | Verify App and KeyValue reconciliation in the isolated stack — **DONE**       | 30m | t002       |
| t004 | Simplify the operator watch fix — **DONE**                                    | 20m | t003       |
| t005 | Add durable cache-startup and reconciliation regression coverage — **DONE**   | 45m | t004       |
| t006 | Close out with production recovery and deploy acceptance           | 30m | t005       |

## Definition of done

With the operator service account still unable to list Secrets cluster-wide, the
manager starts and remains stable, App and KeyValue controllers synchronize, and
namespace-scoped Secret events reconcile their owning KeyValues. `make test` passes
from `lego/operator/`. The corrected operator is deployed to production with no
Secret-list denials or manager restarts, and a fresh `beancount-cms-v2` deploy
creates a build Job and reaches `live`.

## Source + Goal linkage

- **Source:** promoted from the 2026-07-15 production incident note `w10/001`, moved to `w10/done/001.md`; deploy `dep-d9bj8s3eg85c7390eba0` timed out before creating a build Job because the operator could not synchronize its Secret informer.
- **Goal linkage:** restores the core operator mechanism that turns `App` CR intent into running services while preserving ADR028's least-privilege Secret boundary.
- **Expected outcome:** the operator starts reliably without cluster-wide Secret permission, KeyValue Secret ownership events remain functional, and tenant service deploys progress normally again.
- **Why now:** production reconciliation is stopped for every App, and retrying tenant deploys only creates three-minute timeout failures until the controller cache can start.
- **Render parity:** omitted because this is a pure operator-internal reliability and RBAC correction; it changes no REST, GraphQL, MCP, or dashboard contract.
