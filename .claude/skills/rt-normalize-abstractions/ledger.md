# rt-normalize-abstractions ledger

Earlier runs' **Deferred** and **Skipped** findings, read at the start of every run and updated by its change (see the ground rules in [SKILL.md](SKILL.md)). One line per finding, newest first, at most 40 entries.

## Deferred

- 2026-11-15 `dashboard/.../database-plan-section.tsx` — recomposes `postgres.CheckDatabaseAdmission` (HA, `maxStorageGB`, replica cap, autoscaling, pooler) to disable Save; needs an admission-verdict API field
- 2026-11-15 `databases/lib/storage.ts` — mirrors `storageStepGB = 5` and grow-only, omits the 6-hour `CheckStorageIncreaseWait` cooldown, so the UI can offer a resize the server refuses; needs the next-allowed time from the API
- 2026-11-15 `workspaces/types.ts` plan gates (`proFeaturesPlan`, `inboundIpRulesPlan`) — re-encode `core.planFeatures`/`PlanAllows`; needs a per-feature allow list on `Workspace`
- 2026-11-15 `services/lib/instance-type.ts` — mirrors `apps.paidOnlyServiceType`; `instanceTypes` takes no service-type argument
- 2026-11-15 `metrics/lib/range.ts` 720 h cap — mirrors the `BEX_MAX_QUERY_HOURS` deployment setting
- 2026-11-15 mobile `notifications/destination-access.ts` — copies `notifications.destinationRequiredRelation`; needs the relation on `PushNotification`
- 2026-11-15 mobile `deploys/deploy-action-eligibility.ts` — implements `RollbackEligible` but not `RollbackActionable`, so it can pick the live deploy; the projection lacks a target id
- 2026-11-15 mobile `common/apollo/access-link.ts` — hand-maintained verb→relation map (fail-closed; bex-api re-authorizes)
- 2026-11-14 dashboard spend caps (`agent-sessions/lib/spend-limit.ts`, `agent-environment/lib/spend.ts`), free-DB expiry (`databases/lib/expiry.ts`), Postgres versions and identifiers, push-policy validation, webhook SSRF table — each mirrors a server rule; needs an API answer or a shared corpus family
- 2026-11-14 `sshgateway/modelproxy` and `egressproxy` → `agentcred`, `sandboxfiles` → `sandboxsse` — transport→transport imports; the shared pieces belong in the `sshgateway` kernel
- 2026-11-14 `cmd/ssh-gateway` → `internal/api` for `NewTenantService` — move tenancy to a feature package implementing `core.WorkspaceResolver`

## Skipped

- 2026-11-15 `common/lib/ip-allow-list.ts` `isAllowAllRule` — display label derived from rules the user sees; matches `core.RestrictsServiceInbound`
- 2026-11-15 client/server validation parity via `lego/backend/testdata/validation/*.json` and the read-replica name corpus — already guarded by shared fixtures
- 2026-11-15 a `lego/cli` deny in operator/backend `module-dag` — Go already rejects every such import (`package main` root, `internal/` subpackages)
- 2026-11-14 `internal/api/tenancy.go` in the composition root — documented exception that keeps `core` out of the store dependency
