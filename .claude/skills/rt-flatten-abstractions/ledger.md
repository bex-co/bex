# rt-flatten-abstractions ledger

Earlier runs' **Deferred** and **Skipped** findings, read at the start of every run and updated by its change (see the ground rules in [SKILL.md](SKILL.md)). One line per finding, newest first, at most 40 entries.

## Deferred

None yet.

## Skipped

- 2026-11-15 `agentsessions.QueueSessions` — the dispatcher test implements it (`countingSteer`); flattening forces a non-mechanical test rewrite
- 2026-11-15 `types.DiskChildName`, `expandDest` — semantic naming
- 2026-11-14 operator `sandboxMeterCredsMinter`, `datastoreLister`, `clusterBuilderGetter`, `staticserver.Origin`/`Resolver`, `migrate.Registry`/`Objects`, `portscan.Scanner` — documented fake-based test seams
- 2026-11-14 backend `store.PGStore` narrow interfaces and the OpenFGA/OpenBao/Stripe/K8s/S3/pgx seams — external-dependency inversion
- 2026-11-14 `CronJobName`, `BuildJobName`, `PreDeployJobName`, `PubliclyRoutable`, `SafeDialContext`, `k8sname` helpers — cross-module naming contracts (rt-normalize territory)
- 2026-11-14 types `DeployReasonPartTemplates`/`DeployReasonTemplates` test-only exports, `EffectiveNotificationsToSend` — removing them churns the `types` contract for marginal gain
- 2026-11-14 CLI `instructionCommandFor`, `packageManagerHint`, the `deps`/`system`/`upgrader` DI structs — test seams
- 2026-11-14 dashboard `useEventLane`, `isEnvGroupNotFound`, the two `useConnectionInfo` binders, fixed-prop skeleton wrappers — idiomatic accessors, tested domain aliases, TanStack Router constraints
- 2026-11-14 operator `growOnlyStorage`, `uncachedSecretClient`, `commonHTTPPorts`/`defaultHTTPClient` — the name carries intent the one-line body does not
