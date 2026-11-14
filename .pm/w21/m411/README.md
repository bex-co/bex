# w21 · m411 — Operator hardening from the round-25 audit: the PROXY trust list, the edge cache, the static server's fetch leader, registry-migrate, logging, disk snapshots and the build context

**Worker:** worker21 **Goal:** each gap the round-25 audit found in the operator's proxies, jobs and logging is fixed with a test that fails first, or recorded as accepted with the reason. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](t001.md) | PROXY trust: a trusted CIDR with host bits set is refused, not widened | 30m | — |
| [t002](t002.md) | Edge cache: idle and header limits that keep streaming and long-poll responses working | 30m | — |
| [t003](t003.md) | Static server: a cancelled fetch leader no longer fails the requests waiting on its fetch | 45m | — |
| [t004](t004.md) | registry-migrate: credentials leave argv | 30m | — |
| [t005](t005.md) | Logging: the manager and the static server stop running zap in development mode | 20m | — |
| [t006](t006.md) | Disk snapshots: no 78 GiB ceiling, a restore key that cannot contain `..`, and the wrong `os.Root.Symlink` comment | 75m | — |
| [t007](t007.md) | Docker builds: the context and Dockerfile directories are checked for a link out of the source mount in the BuildKit step, not only when there are secret files | 45m | — |
| [t008](t008.md) | Simplify | 15m | t007 |
| [t009](t009.md) | Test coverage | 20m | t008 |
| [t010](t010.md) | Closeout | 10m | t009 |

## Definition of done

1. A trusted-proxy CIDR with host bits set is refused at startup, and the trusted-peer-without-header behavior is recorded as accepted.
2. The edge-cache sidecar has an idle timeout and a header bound that leave streaming and long-poll responses alone.
3. A cancelled static-server fetch leader does not fail the requests that coalesced onto its fetch.
4. `registry-migrate` passes no password in argv.
5. The manager and the static server log production JSON by default.
6. A disk of up to 10,000 GB snapshots within the job's memory, a restore key with `..` is refused, and the symlink comment is true.
7. A Docker build refuses a context or Dockerfile directory that is a link out of the source mount, with or without secret files.
8. Each is a test that fails first, and `make test` and `make lint` (from `lego/operator/`) pass.

## Source + Goal linkage

- **Source:** inbox note w21/073 (promoted 2026-11-14), from the w21/m394 round-25 audit, hardening (Proxies and jobs, Logging).
- **Goal linkage:** the security-review lineage (ADR028 to ADR110) and the operator's tenant-facing daemons.
- **Expected outcome:** none of the seven items remains a gap a later change could turn into a widened trust list, a dropped request, a leaked credential or a build that reads the builder's filesystem.
- **Why now:** the audit's evidence is fresh, and two of the items (the 78 GiB ceiling and the leader cancel) are failures a customer meets without any attacker.
- **Render parity task:** omitted. Operator-internal hardening; no REST, GraphQL or MCP behavior and no customer-facing Render behavior changes, except that disks the API already accepts (up to 10,000 GB) will now back up.

## Open decisions

None.
