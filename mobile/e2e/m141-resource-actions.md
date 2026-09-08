# m141 resource-action consumption verification

Verified 2026-09-08 on the local implementation. These are type, unit, and
bundle checks; simulator interaction and physical-device qualification were
not run in this session and remain in their existing w11 tasks.

## What changed

Mobile operational controls now consume the ADR087 per-resource action
projections (`serverActions`, `deployActions`, `databaseActions`,
`keyValueActions`) instead of combining workspace grants with parallel
lifecycle/status predicates:

- `capabilities/api/resource-actions.graphql` — four typed queries selecting
  only `action`/`outcome`/`reason`/`precondition` (codegen-owned).
- `capabilities/resource-actions.ts` — fail-closed normalizer (`toResourceSnapshot`,
  `resourceDecision`, `isExecutable`, `blockedPrecondition`), shared
  presentation (`presentAction`: denied/unavailable/missing → hidden,
  ready → enabled, blocked → disabled with reason; protected confirmation
  stays enabled for the server-phrase round trip), and `blockedReasonKey`.
- `capabilities/api/use-resource-actions.ts` — workspace+target-bound hooks
  that only bind completed responses for the current variables and refetch on
  the m139 access generation.
- Consumers: `ServiceActionsCard` (lifecycle + trigger/cancel/rollback),
  `CronRunsCard` (run/cancel), Postgres and Key Value detail screens. Denied
  actions are absent; permitted-but-blocked actions stay visible with bilingual
  copy (`resourceActions.blocked.*` in en/zh); cancel without a server-known
  open row stays absent. Controllers bind the confirm-time server gate into
  the request and fingerprint, so a flipped outcome/precondition fails in the
  controller instead of reusing the earlier confirmation; the verbs still
  re-run every guard at dispatch. `SafeActionPanel` renders `disabledReason`
  and refuses to confirm a disabled option.
- Removed: `serviceLifecycleCapabilities` phase/type predicates,
  `serviceSuspended` controller inputs, client cancelable/rollbackable gating,
  and the `can_create` workspace-grant rollback gate (the server probes
  `can_create` per rollback decision). Status sets remain as history-row
  selectors only. No backend, dashboard, REST, GraphQL-schema, or MCP change.

## Acceptance evidence

| Requirement                                                  | Evidence                                                                                                                                                                                                                                                                                                                                          |
| ------------------------------------------------------------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Four projections consumed, fail closed on unknown vocabulary | `resource-actions.test.ts`: unknown action/outcome/reason/precondition never enable; cross-workspace/resource reuse refused. `resource-actions-documents.test.ts`: exact minimum field sets, no sensitive fields. Scope-policy inventory updated for the four queries and still denies sensitive/destructive controls.                            |
| Service/deploy/cron/datastore reflect server eligibility     | Controller/eligibility suites rewritten to the gate contracts: ready gate sends regardless of cached status; denied/blocked/missing gates never send; changed gate under a reused confirmation id rejects. Lifecycle suite proves sends only with a ready decision and keeps the protected-phrase round trip.                                     |
| Confirmation binds target/action/generation, no replay       | Fingerprints include the server gate (`outcome:precondition`); `SafeActionPanel` drops selections whose option left the list and refuses disabled options; hooks refetch on the m139 generation; post-action refresh re-resolves preconditions alongside data. Double-tap single-flight suites still pass.                                        |
| Projection/execution agreement, bilingual reasons, checks    | Blocked-precondition → copy-key mapping covered per code; en+zh strings added; `format:check`, `typecheck`, `lint` (ESLint + knip), `test:unit` (361 passed, plus the access-recovery node suite), `expo:check`, `bundle:ios`, `bundle:android` all pass (rerun after the final production change). No backend change, so no backend suite delta. |

## Render parity (t004)

ADR018 Suspend/Resume, Restart, deploy trigger/cancel/rollback, and
managed-datastore lifecycle rows are complete and untouched: no REST, GraphQL
schema, MCP, dashboard, or verb change shipped here, so no completed cell is
newly implemented or misreported and no drift was filed. Deliberate native
presentation differences (recorded, not drift):

- Denied actions are hidden on native (m138 role views) where the dashboard
  disables with a reason — same authorization, different placement.
- Temporarily blocked actions stay visible but disabled with bilingual copy,
  mirroring the dashboard's explicit-reason pattern.
- Cancel with `no_active_deploy`/`no_active_run` stays absent: the terminal
  history itself is the explanation and there is no target row to name.
- Rollback names the most recent live/deactivated row from loaded history;
  the server scans the recent 20 for eligibility.
- Protected-environment suspend stays an enabled option; the server phrase is
  the explicit second confirmation step through the existing mutation error
  contract — no new flow, no automatic replay.
- While either projection is unresolved for the exact target, the card shows
  no options (fail closed) rather than guessing from cached state.

## Limits

- No simulator or physical-device interaction was exercised this session;
  delivery/signing/live-agent gates stay in w11.
- Projection/execution agreement is proven at the contract level (shared
  predicates, gate-shaped tests), not against a live server round trip.
