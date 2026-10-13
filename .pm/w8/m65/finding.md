# PostgreSQL logs silently omit distinct burst records

Why: successful database work can lose most of its diagnostic records before customers search them through the hosting logs interfaces.

Severity: major. Attribution: infrastructure / CNPG process-log sampling; exact deployed manager identity is unverified.

## Actual native reproduction and controls

Published Bex0.3.2 and unmodified Render2.27/a764810a7682, human QA grant, bex-canary / tea-daif693dqjvc73e7as3g, Free PG18, non-TTY JSON. One unique owned fixture is deleted after testing. [Complete commands/stdout/stderr/exit/time and cleanup](evidence/cli-sweep218/owned-native-SQL-history-cleanup.json).

Using native psql, run a finite DO block that loops over 1..N and emits each numbered message with RAISE LOG. No database rows or platform configuration are written. The Render N=1005 statement exits0 with empty stderr; after full ingestion only109 records remain (1–100,200,300,…1000), missing896. Independent Bex N=300 also exits0 and retains102 (1–100,200,300), missing198. The immediate first Bex query was empty before ingestion; subsequent full cursor reconciliation proves102, not total loss. [Two independent burst records](evidence/cli-sweep218/burst-Bex-reproduction.json).

Both clients' native log views agree byte-for-byte and by ID with the API's retained subset. A separate N=1005 series paced at 0.025s retains every record across eleven nonoverlapping supported pages. Native prefix/iota/lambda searches behind that page pass3/2/1 through both clients. This rules out the m58 raw-page coverage defect and demonstrates rate-dependent source loss, not CLI limit truncation. Explicit own IPv4 and own-CA controls keep the pre-existing default-IPv6 issue outside this reproduction.

## Root and concrete fix boundary

[CNPG's merged process-logging fix](https://github.com/cloudnative-pg/cloudnative-pg/pull/11131) explains the same first100/every100 pattern: a common record envelope causes the instance-manager logger to sample otherwise distinct database messages. Its [manager startup change](https://github.com/cloudnative-pg/cloudnative-pg/pull/11131/files) disables sampling for pod process subcommands. [Fixed release notes](https://cloudnative-pg.io/docs/1.29/release_notes/v1.29/) confirm availability; select a compatible fixed release rather than assuming the latest minor suits this cluster.

Bex owns the immutable mirrored chart at deploy/gitops/base/cnpg-operator.yaml:15–17. Current target is sha256:67a2cb6e865168d0c211c76c1a10d4369c6465dac7ebda9172cb72bc46217a78. Resolve the chart's actual manager release, compare deployed instance-manager identities, then pin a compatible patched artifact or correct demonstrated rollout drift. An anonymous isolated Helm read timed out; no exact installed or chart version is invented. Native SQL wire and direct manager binary inspection are unexercised. Do not rewrite Bex CLI requests or the already-correct history pager.

## Scope, dedupe and acceptance

All managed PostgreSQL process-log consumers share this source, including primary/replica/pooler/backup process variants where their logger versions are affected. Those sibling variants and platform databases are untested; inventory them before rollout. REST/GraphQL/MCP/dashboard read the same retained source, but only native CLI/REST are exercised here. No live cross-surface loss is asserted.

Search of open and done PM records for process/instance-manager sampling, first100 logging and upstream11131 found no matching implementation issue; broader sampler mentions concern analytics/metrics or unrelated static-routing cases. m58 fixes searches over stored records and remains completed; m61's inherited Node entrypoint and m63's initial-hook lifecycle are distinct.

Acceptance requires fresh owned Free PG bursts through both native clients to retain every numbered record, with genuine hasMore/cursors and exact bytes/IDs, plus normal-rate and Unicode controls. Confirm installed manager identities contain the upstream correction, prove old behavior fails the runtime regression, preserve datastore/backup/replica compatibility and delete every owned fixture. Merely emitting SQL successfully or pacing around loss is not acceptance.

The fix is scheduled as m65 (seven tasks,150m). No QA patch, global operator deployment, commit, push or external issue submission occurs.
