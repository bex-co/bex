# w6 · m13 — live-prod verification evidence (recorded 2026-07-13)

Captured against **real prod** (`api.bex.co` / `dashboard.bex.co` + `kubectl` on the Hetzner app cluster via `scripts/fetch-app-kubeconfig.sh`). This is the first of two independent verification passes this milestone ran (see `README.md`'s "Two independent verification passes" for how they were reconciled); a second pass reproduced every result below with its own scratch identities/screenshots and additionally found BUG 3 (documented in `README.md`, not here).

**Update — BUG 1 and BUG 2 below are now fixed and regression-tested**, contradicting this file's original "both unfixed" framing (left below for the historical repro detail, which is still accurate): `store.AcceptInvitesForEmail` enforces plan limits at accept time (`lego/backend/internal/store/members.go`), and `deploy/gitops/base/bex-api-apps-rbac.yaml` grants the missing `jobs.batch` read. Both are written to the working tree, uncommitted, awaiting `/ship` — see `README.md` for the full fix description and regression tests.

## t001 — ship gate: **already shipped; prod IS running the fixes** (board premise was stale)

The task assumed `42e139f` was undeployed and the m12 diff uncommitted. Neither is true any more:

- `42e139f` (m11 fixes) and `8d6c30c` (m12) are both ancestors of `origin/main` **and** of `8bc5ca4`, the commit whose build prod is pinned to (`02a7f64`: "pin operator+dashboard to 8bc5ca432a1c").
- Running prod images == the pinned digests:
  - `bex-api`, `bex-controller-manager`, `bex-bex-static-server` → `ghcr.io/bex-co/bex-operator@sha256:99e1a54ca5ec…`
  - `dashboard` → `ghcr.io/bex-co/bex-dashboard@sha256:552cc07bdf33…`
- The two config-level m11 fixes verified **directly on the live cluster**:
  - `BEX_KRATOS_ADMIN_URL=http://kratos-admin.auth.svc:80` on the running `bex-api` (was the broken `:4434`).
  - `kubectl auth can-i list|delete keyvalues.app.bex.co --as=…:bex-api` → **yes / yes** (was the 403).

No `/ship` was needed. ✅

## t002 — orphaned `m11-purge-test`: deleted

**Update:** this file originally reported the orphan as already gone at the time this pass reached t002 — a race with the second verification pass, which deleted it moments earlier (`kubectl delete app m11-purge-test -n default`, confirmed clean: no leftover Deployment/Service/pod/Ingress). By the time either pass re-checked, `kubectl get app m11-purge-test -n default` → `NotFound`. Net result unchanged: the orphan is gone. ✅

## t004 — workspace delete + the literal OpenBao black-box re-read ✅

Two fresh scratch identities over `https://api.bex.co` (Kratos session via `X-Session-Token`; workspace verbs are GraphQL-only).

| step | result |
| --- | --- |
| A: create `m13-purge-test` (`private_service`, `nginx:alpine`) | 201, `ownerId` = A's tenant |
| A: `PUT .../env-vars [{key:PURGE_CHECK,value:leaked-if-not-purged-3bbba0ea}]` → `GET` | 200, value present |
| A: `deleteWorkspace(confirmation:"sudo delete workspace m13-purge-a")` | **200, no `keyvalues.app.bex.co is forbidden`** — m11's live RBAC failure is gone |
| App CR after delete | **`NotFound`, no Deployment/pod** — `apps.WorkspacePurger` works; the m11 orphan bug cannot recur |
| B: create the **same-named** service | **201** (in m11 this was 403 — the orphan squatted the name) |
| B: `GET .../env-vars` | **`[]` — zero env vars; A's `PURGE_CHECK` did NOT leak** |

This is the literal black-box re-read m11 was blocked from running. Both scratch workspaces were then deleted; prod is clean (only `agentmarketcap`, `beancount-cms`, `eden-cms-v2`, `hello-static` remain).

## t003 — Team page renders email-primary ✅

- API layer: `workspaceMembers` returns `email: m13-c-486fb6@example.com` (m11 saw this empty — the enrichment failing closed on the wrong port is what made the page fall back to the raw `own-…` id).
- UI layer: real browser on `dashboard.bex.co` → Settings → Team shows **the email as the bold primary line**, Kratos subject UUID as the secondary line, for both members, plus a pending-invite row.
- Screenshot: `.playwright-mcp/m13-team-page-email-primary-live.png`.

## t005 — plan change live on prod ✅

| step | result |
| --- | --- |
| Hobby: invite a 2nd member | **gated** — `bad request: the hobby plan is limited to 1 workspace member(s); upgrade to invite more` |
| `changeWorkspacePlan` hobby → pro | 200, `plan: pro` |
| Pro: same invite | **succeeds** (admin + a `developer` invite, a pro-only role) — upgrade unlocks invites |
| Pro → hobby with 2 accepted members | **guard refuses**: `bad request: workspace has 2 members, exceeds hobby plan's limit of 1` — matches `docs/render-artifacts/workspace-plan-change.md`'s `"%w: workspace has %d members, exceeds %s plan's limit of %d"` |
| plan after the refusal | still `pro` — refused, not partially applied |

Verified at both the API layer and in the real browser (the dialog shows _"Couldn't change plan / bad request: workspace has 2 members, exceeds hobby plan's limit of 1"_). Screenshot: `.playwright-mcp/m13-downgrade-guard-live.png`.

---

# Two NEW bugs this verification surfaced (both now fixed — see the update note at the top of this file)

## BUG 1 (serious) — plan limits are bypassable through the pending-invite window

**Proven live on prod:** a workspace on the **hobby** plan (member cap = 1) ended up with **2 accepted members**.

Repro (each step observed):

1. Workspace on `pro` → `inviteWorkspaceMember` a 2nd member (and a `developer`, a role hobby forbids). Both invites are created **pending**.
2. `changeWorkspacePlan` pro → hobby → **succeeds**: `ChangePlan`'s downgrade guards (`lego/backend/internal/workspaces/service.go:458/467/477`) count rows in `tenant_members` — they never look at pending `tenant_invites`.
3. The invitee makes their first authenticated API call → `api/tenancy.go:acceptInvites` → `store.AcceptInvitesForEmail` (`store/members.go:210`) redeems the invite into a `tenant_members` row with **no plan check at all** — no member cap, no `RoleAllowedOnPlan`.
4. Result: hobby workspace, 2 members. A pending `developer` invite would likewise land a hobby workspace with a role `RoleAllowedOnPlan` forbids.

Two-sided fix, of which the second half is applied here (the first is filed as a follow-up, `w6/011.md`):

- `workspaces.Service.ChangePlan`: count **pending invites** alongside members in the downgrade member-count guard, and check invited roles in the role guard. **Not done in this milestone** — the accept-time enforcement below already closes the invariant (a plan-violating membership can no longer be created), so this is UX polish (fail the downgrade early instead of silently leaving invites stuck), not a correctness gap. Filed as `w6/011.md`.
- `store.AcceptInvitesForEmail` / `api.acceptInvites`: enforce the target workspace's plan **at accept time** (skip/leave-pending an invite that would exceed the cap or carry a disallowed role) — the true enforcement point, and it also closes the race for any state change, not just downgrades. Fails closed without ever blocking a login (accept is on the auth hot path). **Applied**: `lego/backend/internal/store/members.go`'s `planAllowsJoin`, regression-tested by `TestAcceptInviteRespectsPlanLimits` in `store_pg_test.go`.

## BUG 2 — bex-api cannot list `jobs.batch`: usage `build_seconds` metering is broken on prod

`kubectl -n bex-system logs deploy/bex-api` is looping on:

```
usage: build_seconds list jobs for <svc>: jobs.batch is forbidden:
User "system:serviceaccount:bex-system:bex-api" cannot list resource "jobs" in API group "batch" in the namespace "default"
```

`kubectl auth can-i list jobs.batch --as=system:serviceaccount:bex-system:bex-api` → **no**. This is the *same species* as m11's `keyvalues` gap: the `api-role` ClusterRole (`lego/operator/config/api/rbac.yaml`) was never granted `jobs`, so the usage rollup's build-seconds component (docs/ADR023-usage-metering.md) silently fails closed on every reconcile — build-seconds usage is under-reported platform-wide. **Applied:** a read-only `batch/jobs` rule on `deploy/gitops/base/bex-api-apps-rbac.yaml`, the one-line RBAC addition mirroring the `keyvalues` grant.

---

# Reconciling with the second verification pass

This verification ran concurrently with a second agent session working the same milestone in the same working tree and against the same prod cluster — the two together are what "two independent verification passes" in `README.md` refers to. In real time, this pass observed the other's in-flight edits (`api/tenancy.go`, `core/http.go`, `workspaces/service.go` gaining a `TenantResolutionInvalidator` stale-cache fix — BUG 3 in `README.md`, a different bug from BUG 1/2 above) and, to avoid clobbering in-flight work, deliberately edited only the disjoint files this pass owned (`store/members.go`, `store_pg_test.go`, `bex-api-apps-rbac.yaml`). Both passes have since finished; their diffs are disjoint and both are folded into `README.md`'s single final record. The `m13-purge-test` App this note originally flagged as the other session's leftover prod resource has since been deleted (that session's own workspace-delete cleanup, per `README.md`).
