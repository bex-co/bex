# m146 verification — 2026-10-02

## Implementation and regression evidence

- Initial hook regression baseline: 9 failing / 3 passing. Plain transport-error refreshes already failed closed; partial GraphQL responses, stale workspace/generation rebinding, obsolete responses and late confirmations did not.
- Initial component baseline: 22 new failures / 37 existing passes. Final focused component checks: 89 passing, including protected retry, a late protected response after access loss, and revocation between chosen-commit deployment and its auto-deploy follow-on.
- Hook/list integration checks: 90 passing; a real Apollo client and real deploy list/row components show 20 enabled rollback controls sharing exactly one service-summary request. Per-click checks remain independent.
- Composed backend GraphQL tests preserve omitted-owner defaults, return current shared-workspace Viewer/Operator decisions, and refuse mismatched/nonmember workspace selections without returning action decisions. Offline schema generation reconciled only these operations.
- Simplify reviews covered reuse, quality and efficiency. Reused the existing receipt freshness predicate, constructed current confirmation context once, renewed timestamped test fixtures per test, and removed the per-row request multiplication. Kept the row/settings protected workflows local because their surface/action state differs.
- Initial full dashboard run overlapped final edits and exposed header fixtures without an Apollo client. Fixtures now exercise the real confirmation hook with mocked network rows; all 76 tests in the three affected files pass. Final full gates are recorded below once complete.

## Render parity

Official [workspace roles](https://render.com/docs/team-members#role-permissions) and [manual deploy behavior](https://render.com/docs/deploys#manual-deploys) were checked on 2026-10-02. Viewer actions remain read-only, and permitted Contributor bare deploys remain usable. Restart is a deploy; a chosen-commit deploy retains the auto-deploy-off follow-on. These GraphQL resource-action projections and client receipt bounds are bex extensions, not Render public endpoints. REST/MCP mutations keep their existing shared authorization.

The Render matrix permits Contributor rollbacks. Bex's existing `can_create` requirement for rollback/executable selection is a deliberate stricter ADR024 boundary; it remains explicitly documented in ADR018 rather than being described as full parity or silently weakened here. No new role model or endpoint family was added.

## Live evidence and limits

Used only dev-6 with real Kratos owner/member cookie sessions, isolated real OpenFGA, a Scale workspace, and an image-backed managed service. The operator reached Ready and the fixture reached Running/Ready, including a second live revision.

- Real owner/member sessions and membership invitation succeeded; member capabilities were Contributor with operate allowed and create denied.
- Both owner/member action projections originally returned `not found` because GraphQL omitted acting-workspace selection. After the adapter fix, an authenticated shared-workspace Viewer received denied deploy/cancel/rollback decisions with `insufficient_permission`.
- A real owner PATCH changed the member to Viewer; a fresh capability response confirmed the role and denied operate/create/sensitive/log grants.
- The desktop mounted header showed disabled Deploy latest image and Restart controls with zero recorded browser mutations. Auth/projection availability remained intermittent, so this alone is not evidence of every denial reason or every required state.
- Full allowed/denied/unavailable browser coverage, narrow-mobile geometry, poll/focus/reconnect, late-response/open-confirmation races, protected retries, actual removal/personal fallback, and the separately labeled empty-membership injection remain unverified live. Mounted regression evidence does not substitute for those checks. No production walkthrough was run.

## Local infrastructure gate

The CAPD API recovered briefly, but repeated API/port-forward timeouts coincided with host load around 100 and heavy unrelated CI activity. Dev-6 auth database 5s liveness checks repeatedly timed out, starting 180s smart Postgres shutdowns while connections remained pooled. The operator also lost its leader lease after API PUT timeouts; this was not an application panic.

Initial recovery rebuilt the current operator image, loaded it onto the existing three local nodes, and restored the mock baseline/CRDs/operator RBAC; the cluster and its data were not rebuilt. Scoped recovery: refreshed own Kratos/admin/DB tunnels; pinned only dev-6 Kratos/Hydra/courier to the local control-plane node; patched only dev-6's three CNPG Clusters to 15s liveness/readiness timeouts, liveness failure threshold 6, and 30s smart shutdown. PVCs and data were preserved. No other dev-N workload was modified, no production context was used, and no node/container restart or reprovision was attempted during this resumed failure. The dashboard and API listeners were stopped and the browser left on a blank page to reduce retries while acceptance is gated.

## Fixture cleanup

Cleanup blocked by auth/database availability; these fixtures remain and must be removed after the walkthrough. Own fixture inventory (no credentials): shared workspace `tea-db01ldhjg4r41b7e7lng`; service `srv-db01lt9jg4r41b7e7lqg`; owner personal workspace `tea-db01la1jg4r41b7e7lm0`; member personal workspace `tea-db01lkhjg4r41b7e7lpg`; Kratos identities `fe0d89b8-1259-439e-af90-650d66eb05a4` and `aadfdd30-4667-4242-b660-53264f90546e`. Fixture name prefix: `w6-074-1790974606675`.

## Final gates

- Backend `GOWORK=off go test -p 1 ./...` passed against fresh dedicated Postgres/OpenFGA/OpenBao services; `make lint-backend` passed with zero issues.
- Dashboard full lint (including both TypeScript projects and unused-code analysis) passed after the final fixture corrections. Final full dashboard run: **458 files / 3,912 tests passed**.
- Dev-6 live acceptance and fixture API cleanup remain gated: the final bounded recovery check found all three databases unready, Kratos timeout/503, Hydra 503, and host load above 120. Stopped this run's local API/dashboard listeners to reduce retries; own fixture data and the isolated `bex-w6-074-fga` container on port 55477 remain for recovery. No unrelated CI runner or dev stack was changed.

After syncing `origin/main` through `dab8227fa`, affected dashboard checks passed again (50 files / 480 tests), and backend API/apps/deploys/Postgres package tests passed. These post-merge package checks used ordinary unit configuration; the complete real-service backend run above preceded the sync. Shared GraphQL generated types merged without conflicts, preserving both workspace arguments and incoming Postgres fields.

Full dashboard lint also passed on the merged tree, including both TypeScript projects, ESLint and unused-code checks.
