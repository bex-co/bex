# rt-normalize-abstractions ledger

Earlier runs' **Deferred** and **Skipped** findings, read at the start of every run and updated by its change (see the ground rules in [SKILL.md](SKILL.md)). One line per finding, newest first, at most 40 entries.

## Deferred

- 2026-11-15 `dashboard/.../databases/components/database-plan-section.tsx` — recomposes `postgres.CheckDatabaseAdmission` (HA, `maxStorageGB`, replica cap, autoscaling, pooler) to disable Save; needs an admission-verdict API field
- 2026-11-15 `databases/lib/storage.ts` — mirrors `storageStepGB = 5` and grow-only, omits the 6-hour `CheckStorageIncreaseWait` cooldown, so the UI can offer a resize the server refuses; needs the next-allowed time from the API
- 2026-11-15 `workspaces/types.ts` plan gates (`proFeaturesPlan`, `inboundIpRulesPlan`) — re-encode `core.planFeatures`/`PlanAllows`; needs a per-feature allow list on `Workspace`
- 2026-11-15 `services/lib/instance-type.ts` `offeredInstanceTypes`/`offersFreeTier` — mirror `apps.paidOnlyServiceType` (the server enforces it via `errFreePlanForType`; the two sets agree today, with no test comparing them); needs the offered plans per service type from the API
- 2026-11-15 `metrics/lib/range.ts` 720 h cap — mirrors the `BEX_MAX_QUERY_HOURS` deployment setting
- 2026-11-15 mobile `features/notifications/destination-access.ts` — copies `notifications.destinationRequiredRelation`; needs the relation on `PushNotification`
- 2026-11-15 mobile `features/deploys/deploy-action-eligibility.ts` — implements `RollbackEligible` but not `RollbackActionable`, so it can pick the live deploy; the projection lacks a target id
- 2026-11-15 mobile `common/apollo/access-link.ts` — hand-maintained verb→relation map (fail-closed; bex-api re-authorizes)
- 2026-11-14 dashboard spend caps (`agent-sessions/lib/spend-limit.ts`, `agent-environment/lib/spend.ts`), free-DB expiry (`databases/lib/expiry.ts`), Postgres versions and identifiers, push-policy validation, webhook SSRF table — each mirrors a server rule; needs an API answer or a shared corpus family

## Skipped

- 2026-11-18 `lego/backend/internal/sshgateway/agentattach` `applyCORS` — documented transport-specific adaptation of `internal/api` `withCORS` (SSE ticket headers vs cookie session), not a shared rule; converging them would mislabel either surface
- 2026-11-16 `dashboard/.../usage/lib/charges.ts` — display math on backend-rated totals (parse/format/linear projection); no rate or tier logic
- 2026-11-15 `common/lib/ip-allow-list.ts` `isAllowAllRule` — display label derived from rules the user sees; matches `core.RestrictsServiceInbound`
- 2026-11-15 client/server validation parity via `lego/backend/testdata/validation/*.json` and the read-replica name corpus — already guarded by shared fixtures
- 2026-11-15 a `lego/cli` deny in operator/backend `module-dag` — Go already rejects every such import (`package main` root, `internal/` subpackages)
