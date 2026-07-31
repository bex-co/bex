# Workspace-family parity walk — Team · Env Groups · Usage/Billing · Notifications · Webhooks · Blueprints (w5/m59)

The workspace-scoped page family had not had a dedicated parity walk since `w5/m32` (2026-07-15), while a lot shipped onto it since. This is that re-walk.

## Method + honesty note

The m32/m57 walk method is a **live authenticated Render + bex side-by-side** capture. In this session the live side was **infrastructure-blocked**: no authenticated Render account/browser session was available, and the local `dev-5` stack could not be raised (the shared kind cluster is missing the CNPG `postgresql.cnpg.io/v1` CRDs the stack needs) — and no authenticated prod bex session was available either. So this walk is a **code-based (static) parity audit**: for each family it inventories what the bex dashboard actually ships (routes + feature + components) and cross-references it against the **pinned Render artifacts already in this directory** and the `docs/ADR018-render-parity.md` ledger, rather than fresh live captures.

What this catches: missing pages, dashboard-unconsumed capabilities, dead-ends, and stale ADR018 cells. What it **cannot** catch: Render-side visual/interaction drift since the artifacts were pinned (a Render redesign, a new control Render added). That live drift-check is the one deferred DoD element, tracked as open note `.pm/w5/031.md`; run it against the deployed dashboard + a Render account when both are available.

## Per-page verdicts

| Page family | bex route(s) | What bex ships | Render artifact / ADR018 | Verdict (code-side) |
| --- | --- | --- | --- | --- |
| **Team / Members** | `/workspace/settings` (Team panel) | Members table (email/userId/role), invite-by-email dialog (plan-gated role picker), change-role, remove, pending invites (resend/revoke), seat usage ("X of Y seats"), per-member MFA badge, invite-accept on login + token | `owners-api.md`; ADR024; ADR018 "Workspace members & roles" ✅ | **Match** — full CRUD + seats + MFA + resend shipped (w1/m33). No code-visible gap. |
| **Env Groups** | `/env-groups`, `/env-groups/$groupId` | List (cards: name/id/var+file+link counts/created-by), atomic New dialog (envVars incl. generateValue + secretFiles + initial service links), detail editors (env vars + secret files), link/unlink services, rename, delete | `env-group-create.md`, `env-group-list-filters.md`; ADR018 "Environment groups" ✅ | **Match** — atomic populated-create (w5/m33) + list (w5/m26) + detail (w5/m31). No gap. |
| **Usage / Billing** | `/usage`, `/billing/$` (alias) | Month picker (6 mo) + 3-month trend chart; per-service compute/egress/build/storage tables; workspace resource caps; Stripe billing-onboarding card (customer/subscription/payment/tax/lifecycle readiness), "Go to checkout" (hosted session), "View invoices" (hosted Portal); dunning lifecycle alerts | `billing-onboarding.md`; ADR023, ADR040; ADR018 "Usage metering" ✅ | **Match** — usage (w8/m3–m15) + Stripe checkout/portal (w7/m51) + dunning (w7/m52). No gap. |
| **Notifications** | `/notifications` | Per-member deploy-email preferences (Deploy Started/Succeeded/Failed switches), error/loading/empty states. Per-service override lives in Service Settings (`setNotificationsToSend`), by design | `notify-on-fail.md`; ADR018 "Notifications" ✅ | **Match** — member defaults (w3/m9) + email-content parity (w7/m44). Service override is intentionally on the service page, not here. No gap. |
| **Webhooks** | `/webhooks`, `/webhooks/new`, `/webhook/$id/{index,settings}` | List (name/events/enabled/actions), full-page create (name + endpoint URL + grouped event picker with search/tri-state All/counter), mint-once secret reveal, delivery history (All/Successful/Failed), edit (name/URL/events, dirty-gated), inline enable/disable toggle, sudo-name delete | `webhooks-ui.md`; ADR006 § Outbound event webhooks; ADR018 "Outbound event webhooks" ✅ | **Match** — full CRUD + delivery history + event picker (w1/m49). No gap. |
| **Blueprints** | `/blueprints`, `/blueprints/$blueprintId` | List (name/repo/branch/status/updated), detail (metadata + read-only YAML manifest), Validate (dry-run, per-resource errors), Sync (protected-resource typed confirm), status badge | `dashboard-routes.md`, `service-events.md`; ADR018 "Blueprint … dashboard" ✅ | **Match** — list/detail/manifest/validate/sync (w7/m27). Create/edit/delete stay API/Git-first by design. No gap. |

## Intentional exclusions (recorded, not filed as gaps — per `.pm/DO_NOT_DO.md`)

- **Slack notification delivery** — off-roadmap (DO_NOT_DO; compose via an outbound webhook → Slack bridge). Both Notifications and the service override stay email-only.
- **External log/metric drains** — off-roadmap non-goal; not part of any workspace page.
- **Member avatars / `name` trait** — Kratos schema records only email + UUID; avatars are future work, member mutations stay subject-keyed (documented ADR024 divergence).
- **Billing management UI** (invoice editing/refunds/subscription editing) — Render exposes no public billing-management API either; bex's flows are Stripe-hosted (checkout/Portal), API-first.
- **Blueprint create/edit/delete in the dashboard** — API/Git-first by design (Render's Blueprints are also IaC-first); sync-delete/partial-apply and PR previews are documented non-goals.
- Email is plain-text where Render's is HTML (cosmetic, documented in `notify-on-fail.md`).

## Outcome

Code-side: **verified parity** across all six families — every family has a first-class workspace-scoped dashboard page, every relevant ADR018 row is ✅, and the shipping timeline since the last walk (env groups w5/m33, webhooks w1/m49, billing w7/m51–52, members w1/m33, notifications w7/m44) shows active maintenance with no regressions. **No missing pages and no dashboard-unconsumed capabilities were found**, so no sub-hour fixes were applied and no new gap milestones were filed. The single deferred DoD element is the **live Render-side drift check** (note `031`); nothing in the ADR018 ledger was found stale by this code audit.
