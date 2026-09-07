# Workspace-family parity walk — Team · Env Groups · Usage/Billing · Notifications · Webhooks · Blueprints (w5/m59)

The workspace-scoped page family had not had a dedicated parity walk since `w5/m32` (2026-07-15), while a lot shipped onto it since. This records the 2026-07-30 re-walk; it is a historical static inventory, not a current live-parity certification.

## Method + honesty note

The m32/m57 walk method is a **live authenticated Render + bex side-by-side** capture. In this session the live side was **infrastructure-blocked**: no authenticated Render account/browser session was available, and the local `dev-5` stack could not be raised (the shared kind cluster is missing the CNPG `postgresql.cnpg.io/v1` CRDs the stack needs) — and no authenticated prod bex session was available either. So this walk is a **code-based (static) parity audit**: for each family it inventories what the bex dashboard actually ships (routes + feature + components) and cross-references it against the **pinned Render artifacts already in this directory** and the `docs/ADR018-render-parity.md` ledger, rather than fresh live captures.

What this catches: missing pages, dashboard-unconsumed capabilities, dead-ends, and stale ADR018 cells. What it **cannot** catch: Render-side visual/interaction drift since the artifacts were pinned (a Render redesign, a new control Render added). The deferred blanket drift-check (`w5/031`) was retired during the user-authorized 2026-09-06 inbox triage. Its current, family-specific evidence and limits are recorded below; retirement does not turn static evidence into live proof.

## Historical per-page verdicts (2026-07-30)

| Page family | bex route(s) | What bex ships | Render artifact / ADR018 | Verdict (code-side) |
| --- | --- | --- | --- | --- |
| **Team / Members** | `/workspace/settings` (Team panel) | Members table (email/userId/role), invite-by-email dialog (plan-gated role picker), change-role, remove, pending invites (resend/revoke), seat usage ("X of Y seats"), per-member MFA badge, invite-accept on login + token | `owners-api.md`; ADR024; ADR018 "Workspace members & roles" ✅ | **Match** — full CRUD + seats + MFA + resend shipped (w1/m33). No code-visible gap. |
| **Env Groups** | `/env-groups`, `/env-groups/$groupId` | List (cards: name/id/var+file+link counts/created-by), atomic New dialog (envVars incl. generateValue + secretFiles + initial service links), detail editors (env vars + secret files), link/unlink services, rename, delete | `env-group-create.md`, `env-group-list-filters.md`; ADR018 "Environment groups" ✅ | **Match** — atomic populated-create (w5/m33) + list (w5/m26) + detail (w5/m31). No gap. |
| **Usage / Billing** | `/usage`, `/billing/$` (alias) | Month picker (6 mo) + 3-month trend chart; per-service compute/egress/build/storage tables; workspace resource caps; Stripe billing-onboarding card (customer/subscription/payment/tax/lifecycle readiness), "Go to checkout" (hosted session), "View invoices" (hosted Portal); dunning lifecycle alerts | `billing-onboarding.md`; ADR023, ADR040; ADR018 "Usage metering" ✅ | **Match** — usage (w8/m3–m15) + Stripe checkout/portal (w7/m51) + dunning (w7/m52). No gap. |
| **Notifications** | `/notifications` | Per-member deploy-email preferences (Deploy Started/Succeeded/Failed switches), error/loading/empty states. Per-service override lives in Service Settings (`setNotificationsToSend`), by design | `notify-on-fail.md`; ADR018 "Notifications" ✅ | **Match** — member defaults (w3/m9) + email-content parity (w7/m44). Service override is intentionally on the service page, not here. No gap. |
| **Webhooks** | `/webhooks`, `/webhooks/new`, `/webhook/$id/{index,settings}` | List (name/events/enabled/actions), full-page create (name + endpoint URL + grouped event picker with search/tri-state All/counter), mint-once secret reveal, delivery history (All/Successful/Failed), edit (name/URL/events, dirty-gated), inline enable/disable toggle, sudo-name delete | `webhooks-ui.md`; ADR006 § Outbound event webhooks; ADR018 "Outbound event webhooks" ✅ | **Match** — full CRUD + delivery history + event picker (w1/m49). No gap. |
| **Blueprints** | `/blueprints`, `/blueprints/$blueprintId` | List (name/repo/branch/status/updated), detail (metadata + read-only YAML manifest), Validate (dry-run, per-resource errors), Sync (protected-resource typed confirm), status badge | `dashboard-routes.md`, `service-events.md`; ADR018 "Blueprint … dashboard" ✅ | **Match** — list/detail/manifest/validate/sync (w7/m27). Create/edit/delete stay API/Git-first by design. No gap. |

## Historical exclusions (2026-07-30; supersessions noted)

- **Slack notification delivery** — off-roadmap (DO_NOT_DO; compose via an outbound webhook → Slack bridge). Both Notifications and the service override stay email-only.
- **External log/metric drains** — off-roadmap non-goal; not part of any workspace page.
- **Member avatars / `name` trait** — Kratos schema records only email + UUID; avatars are future work, member mutations stay subject-keyed (documented ADR024 divergence).
- **Billing management UI** (invoice editing/refunds/subscription editing) — Render exposes no public billing-management API either; bex's flows are Stripe-hosted (checkout/Portal), API-first.
- **Blueprint create/edit/delete exclusion is superseded.** `w2/m62` and `w8/m21` delivered dashboard create/edit/disconnect, environment prompts, sync review and settings. The old API/Git-first statement is not a current non-goal; PR previews remain excluded.
- Email is plain-text where Render's is HTML (cosmetic, documented in `notify-on-fail.md`).

## Current evidence and disposition (2026-09-06)

The historical “verified parity across all six families” conclusion overstated what a static audit can establish. Environment Groups later disproved that conclusion in a live walk. Current evidence is scoped as follows:

| Family | Later evidence | Limit |
| --- | --- | --- |
| Environment Groups | Authenticated Render/bex walk and resulting fixes closed in `w2/done/m73` (2026-08-18). | Supersedes the historical match verdict. |
| Webhooks | Authenticated Render capture and bex admin/viewer walkthrough in `webhooks-ui.md` and `w8/done/m26` (2026-08-17). | This family's deferred walk is superseded by stronger evidence. |
| Blueprints | Authenticated Render/create research in `w2/done/m62`; dashboard implementation extended by `w8/done/m21`. | No fresh complete two-dashboard walk claimed. |
| Team, Usage/Billing, Notifications | Shipped implementations and focused tests/artifacts remain the evidence. Service-notification four-state save/reload was also verified on production during w5/029 triage. | No fresh full-family authenticated Render/bex comparison claimed. |

The old open-ended audit note `w5/031` was deleted as a stale verification campaign, not as an implemented feature. Its concrete Environment Groups and Webhooks findings already have completed owners, and its Blueprint exclusion became false after later delivery. The remaining rows contain no identified unimplemented defect. The available Render browser was signed out during this triage; the gaps above stay explicit rather than asserting unobserved live parity. Future feature changes should use focused current comparisons, not this historical table as proof of equivalence.
