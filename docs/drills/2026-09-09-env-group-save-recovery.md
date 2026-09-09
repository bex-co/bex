# Environment-group interrupted-save recovery — 2026-09-09

Proves the w4/m98 contract: a content save or clone source lock interrupted after the busy claim can be recovered by another API instance from durable OpenBao evidence, without permanently blocking the group or the workspace list.

## Failure mode

The revision claim stored only `busy`/`generation`. Restoration snapshots lived in the in-memory `groupPatchTxn`. A process exit after the claim left subsequent reads/saves returning conflicts indefinitely; `ListEnvGroupsFiltered` aborted the whole list when any hydrated group was busy.

## Fix

Before claiming busy, patch writes prior/proposed env and file maps under `env-groups/<gid>/op/<opID>/…`. After winning the busy CAS, it publishes a non-secret op record (`id`, `kind`, `phase`, `leaseUntil`, change flags). Phases advance through `admitted` → `env_written` → `files_written`/`committed`. On success the claim returns to idle and op artifacts are cleared.

`ensureGroupOperable` reclaims only after the lease expires (default 2m). Recovery restores prior maps for incomplete phases or finalizes acknowledged `files_written`/`committed` maps. Clone ops only release the source lock. Legacy busy without an op record still fails honestly (`ENV_GROUP_REVISION_CONFLICT` / `ENV_GROUP_RESTORATION_FAILED`).

List/detail expose `availability` (`busy` | `repair_required`) instead of failing the whole collection; healthy peers remain usable. REST JSON includes the field; GraphQL `EnvGroup.availability` mirrors it. The dashboard queries the field and shows a status row when set.

## Automated evidence

```bash
cd lego/backend && go test ./internal/envgroups/ -count=1 -race -timeout 120s
```

Coverage includes expired-patch finalize, admitted restore, active-lease refusal, list with a busy peer, and clone source unlock (`recovery_test.go`), plus existing patch/clone suites.

## Live drill

Intended on `dev-4` (real OpenBao + Kubernetes): admit a patch, kill bex-api mid-write, restart, confirm recovery within the lease bound, and that a second healthy group remains listable. **Not run this session** (no healthy mock-cluster kubeconfig); Core tests are the dated proof. Re-run and append fixture ids + HTTP classes without secret values.

## Scope boundary

No public force-unlock endpoint. Dashboard list/detail request `availability` and show a status row when set; names-only masking and save modes are unchanged.
