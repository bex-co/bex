# w9 · m481 — An API key acts with its creator's current member role in its workspace, as on Render, so Terraform manages webhooks, registry credentials, owner notification settings and protected environments

**Worker:** worker9 **Goal:** a bex API key acts with exactly its creator's current role in the workspace it was minted in, following that role up and down, never more, as Render's keys do; an Admin's key passes the four Admin-only writes Render's Terraform provider makes, and a Developer's key still cannot. **Status:** in progress (t001–t007 and t009–t011 done; t008 done on dev-9, its production step waits for a pin that carries this milestone)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Design: a key acts with its creator's current role in its own workspace; ADR024, ADR012 §8e and ADR018 amended — **DONE** | 45m | — |
| [t002](done/t002.md) | Mint: a new key is seated at its creator's current workspace role, not `developer` — **DONE** | 45m | t001 |
| [t003](done/t003.md) | Follow: every change to a creator's workspace role re-seats their keys through the durable role outbox — **DONE** | 1h30m | t002 |
| [t004](done/t004.md) | Fail closed: a key's decision never exceeds its creator's, under partial failure, pending reconciliation, another workspace or no recorded creator — **DONE** | 1h15m | t003 |
| [t005](done/t005.md) | Webhooks: an API key with a recorded human creator passes webhook create and repoint; every other mint verb stays human-only — **DONE** | 45m | t001 |
| [t006](done/t006.md) | Dashboard: the API keys page says a key acts with your role in this workspace and follows it — **DONE** | 30m | t002 |
| [t007](done/t007.md) | Security review of the key-role boundary, recorded in the security lineage — **DONE** | 45m | t004, t005 |
| [t008](t008.md) | Live check on dev-9 and production; Render's Terraform provider tests rerun; checklist class 3 closed | 1h30m | t006, t007 |
| [t009](done/t009.md) | Render parity — **DONE** | 30m | t008 |
| [t010](done/t010.md) | Simplify — **DONE** | 15m | t009 |
| [t011](done/t011.md) | Test coverage — **DONE** | 45m | t009, t010 |
| [t012](t012.md) | Closeout | 10m | t011 |

Total: **12 tasks, about 9h45m**.

## Security boundary (holds for every task)

- **I1: never more than the creator, now.** For every relation and object, a key is allowed only if its creator is allowed the same thing at the same moment. The key's staleness window is never longer than the one the creator's own session has (the same caches, and the same uncached re-checks before durable writes).
- **I2: its own workspace only.** A key acts only in the workspace it was minted in, whatever role its creator holds anywhere else (Render: "Each scoped API key can only access the workspace you create it in").
- **I3: no recorded creator, no expansion.** A key with no `createdBy` stamp keeps today's Developer binding and never gains a higher role.
- **I4: keys still do not mint credentials.** A key still cannot create an API key, enroll an SSH key, invite or re-role a member, or create an organization or workspace. t005 widens only webhook create and repoint, and the board records every other Render-reachable mint verb as a follow-up (t009).
- **I5: member verbs still refuse key subjects** (`guardMachine`, ADR024 § A member is a person). A key's role changes only through its creator's role.

## Definition of done

- **REST on dev-9, as an Admin's key:** the key creates a webhook (`POST /v1/webhooks`) and a registry credential (`POST /v1/registrycredentials`), patches owner notification settings (`PATCH /v1/notification-settings/owners/{id}`) and sets an environment `protected`.
- **As a Developer's key:** each of those four writes is 403.
- **After a demotion:** once the Admin is demoted to Developer, their key is refused all four on its next request. A failed or pending tuple write does not change that, and tests prove it (I1).
- **GraphQL and MCP** answer the same for the same keys.
- **Render's Terraform provider on dev-9:** run with an Admin's key as minted (no hand-written OpenFGA grant), `render_webhook`, `render_notification_setting` and `render_project` get past their first step, and the supplementary `render_registry_credential` apply passes. [terraform-compatibility-checklist.md](../../../docs/terraform-compatibility-checklist.md) class 3 is closed.
- **On production as the QA user**, once the pin carries this milestone: a freshly minted key creates a webhook and a registry credential and patches owner notification settings, then everything is restored and deleted.
- **Docs:** ADR024, ADR012 §8e and ADR018 record the rule and its security reasoning, and the security lineage records the review.

## Source + Goal linkage

- **Source:** promoted from [w9/107](../done/107.md) (2026-11-18), found by w9/m474's re-recording of Render's Terraform provider suite ([checklist](../../../docs/terraform-compatibility-checklist.md), class 3).
  - Render ([render.com/docs/api-keys](https://render.com/docs/api-keys), read 2026-11-18): "Your member role in the selected workspace determines which permissions you can grant an API key", "Enable all" grants "every permission your member role allows", classic keys "have the same permissions as your member role in every workspace you belong to", and the FAQ "What happens to an API key if my member role changes?" answers "Your key's access adjusts to match your new member role". The Workspaces permissions include "Create, update, and delete webhooks" and "Update notification settings". The Projects permissions include "Configure protected environments".
  - bex (main `2551d3dce`): `lego/backend/internal/tenancy/tenancy.go:612` `BindKey` → `internal/authz/authz.go:241` `GrantWorkspaceMember` always writes `developer`. `internal/webhooks/service.go:531` (create) and `:936` (repoint) call `AuthorizeMintClass`, which refuses every non-human caller (`internal/core/base.go:1170`, w4/079). The four writes are `RelCanManage` (Admin) in `deploy/gitops/authz/model.fga`.
- **Open questions settled here:**
  - **Snapshot or follow:** the key follows its creator's role, because Render's FAQ says so.
  - **Fine-grained per-resource permissions:** not needed to reach the Done-when. They are Render's scoped-key variant, filed as a follow-up in t009 (ADR008: ship the parity-sufficient option).
  - **w4/079's mint-class rule:** it admits an API key with a recorded human creator for webhooks only (t005). w4/079 was about delegated third-party OAuth tokens, which stay refused.
- **Goal linkage:** Render parity first ([ADR008](../../../docs/ADR008-vision.md#render-parity-first)). Render's Terraform provider is a client bex must serve ([ADR018](../../../docs/ADR018-render-parity.md) "Terraform provider" row).
- **Expected outcome:** an Admin applies Render Terraform configurations that hold webhooks, registry credentials, owner notification settings and protected environments, unmodified, with a bex API key.
- **Why now:** this is the class-3 failure behind several of the checklist's remaining `[ ]` rows: webhook, registrycredential, notifications, and every service or datastore test that configures a protected environment. w9/m474 just graded it, and w9/105 and w9/106 already cleared the other first-step failures.
- **Render parity:** included (t009). The change is customer-facing on REST, GraphQL, MCP and the dashboard's API keys page.
- **Security:** this changes what a credential may do, so t007 reviews it in the security lineage, and the boundary above is every task's acceptance floor.
