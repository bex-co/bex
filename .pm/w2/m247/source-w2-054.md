# 054 — A Hobby workspace cannot run a high-availability Postgres: the tenant quota and the barman sidecar's limits refuse the standby

Why: on Render, HA is available on any ≥1-CPU Postgres plan with no workspace-plan gate (its HA and features-by-plan docs, re-read 2026-10-30). In bex, a Hobby workspace's `tenant-quota` refuses the standby, so `enableHighAvailability` on `1c-2g` either never gets a second instance or never fits at all (`2c-4g`). That is a Render-parity failure of exactly what w2/m171 shipped.

Found 2026-10-30 in w2/m245 t005, in production workspace `bex-canary` (Hobby).

## Evidence

- CNPG refused instance `-2` of a `1c-2g` HA database: `pods "…-2" is forbidden: exceeded quota: tenant-quota, requested: limits.cpu=3,limits.memory=6Gi, used: limits.cpu=3100m,limits.memory=6400Mi, limited: limits.cpu=6,limits.memory=12Gi`.
- Each instance Pod is charged **3 CPU / 6 GiB of limits** for a 1 CPU / 2 GiB plan. `postgres` itself has a limit of 1/2Gi, but the CNPG **barman-cloud plugin** native sidecar (`initContainers[plugin-barman-cloud]`, `restartPolicy: Always`) has `limits: cpu 2, memory 4Gi` (requests 100m/128Mi), and Pod-level limits sum them.
- Hobby quota (`lego/backend/internal/store/namespaces.go` `quotaForPlan`): `requests.cpu 3`, `requests.memory 6Gi`, `limits.cpu 6`, `limits.memory 12Gi`. A `2c-4g` HA pair alone needs about 4.2 CPU of requests, so it can never fit in a Hobby workspace. A `1c-2g` pair (about 2.2 requests / 6 limits) fits only in an otherwise empty workspace.
- The same database in the paid `bex` workspace (quota 58 CPU) became healthy with both instances.

## Do

1. Decide how a Hobby workspace accommodates datastores that Render would allow. Render has no per-workspace CPU quota. Options: exclude CNPG/Valkey pods from the compute quota through a PriorityClass-scoped `ResourceQuota` (datastores are already bounded by count caps and paid per instance), or size the Hobby quota so that the largest sellable HA pair plus the existing surge headroom fits. Prefer the parity-sufficient option and record the rationale in `quotaForPlan`'s comment.
2. Set the barman-cloud sidecar's resources explicitly (CNPG plugin `ObjectStore` `instanceSidecarConfiguration.resources`, or the operator's injection) so a 1-CPU instance is not charged 3 CPU of limits. Keep the backup sidecar adequately resourced.
3. Verify on production in a Hobby workspace: `1c-2g` and `2c-4g` HA databases reach two healthy instances, then delete them.

## Acceptance

- [ ] A Hobby workspace can create and run a `2c-4g` HA Postgres (two healthy instances), and an instance Pod's quota charge matches its plan plus a bounded sidecar.

**Promoted 2026-10-30 → [`w2/m247`](README.md).**
