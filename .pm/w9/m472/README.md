# w9 · m472 — `terraform apply` with Render's provider creates services, cron jobs and webhooks on bex, as on Render

**Worker:** worker9 **Goal:** the request bodies Render's official Terraform provider sends, which Render accepts, are accepted by bex, so a minimal Terraform configuration applies, re-plans clean and destroys on bex **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](t001.md) | `serviceDetails.previews: {}` is Render's default (off) on create and PATCH, as Render accepts it | 30m | — |
| [t002](t002.md) | A webhook created with `eventFilter: null` subscribes to every event, as Render stores it | 45m | — |
| [t003](t003.md) | A Docker or image cron job created with `envSpecificDetails.registryCredential` binds its `id`, as Render's provider sends it | 45m | — |
| [t004](t004.md) | Optional request lists Render accepts as JSON null (`secretFiles`, `envVars`, `readReplicas`, `routes`) are accepted as omitted | 45m | — |
| [t005](t005.md) | Production: `terraform apply`, a clean second plan and `destroy` with Render's provider in bex-canary | 1h | t001, t002, t003, t004 |
| [t006](t006.md) | Render parity | 30m | t005 |
| [t007](t007.md) | Simplify | 20m | t006 |
| [t008](t008.md) | Test coverage | 30m | t006 |
| [t009](t009.md) | Closeout | 15m | t007, t008 |

## Definition of done

- On production, as the QA user in bex-canary, Terraform with a pinned `render-oss/render` provider applies a `render_web_service` with no `previews` block, a Docker `render_cron_job` with no registry credential and a `render_webhook` with no `event_filter`; a second `terraform plan` shows no changes; `terraform destroy` removes all three.
- `POST /v1/services` with `serviceDetails.previews: {}` succeeds; `{"generation":"automatic"}` is still refused by name.
- `POST /v1/webhooks` with `eventFilter: null` answers `201` and the webhook receives every event type.
- A cron create carrying the provider's `envSpecificDetails.registryCredential` object answers `201`, binding a non-empty `id`.

## Source + Goal linkage

- **Source:** `/pm-brainstorm more for w9, preferrably one aspect of render parity` (2026-11-17), item 1, filed by `/pmx 1,2,3 to w9`. Evidence from `render-oss/terraform-provider-render` at tip `06af456` (2026-11-16): its current request-building code, and the 46 go-vcr cassettes its test suite replays, which record Render's real API answering these exact requests (staging/testing hosts, 2024-11 to 2026-05). Each task cites the provider file, the recording and the bex line that refuses today.
- **Goal linkage:** [ADR008 § Render parity first](../../../docs/ADR008-vision.md#render-parity-first). Render calls the provider its official one ([render.com/docs/terraform-provider](https://render.com/docs/terraform-provider)); like the Render CLI, it is a client bex must accept, not a product bex builds.
- **Expected outcome:** a customer pointing Render's provider at bex (`RENDER_HOST`) can create resources with the same minimal configuration that works on Render. Today the first service create answers 400.
- **Why now:** w7/m426 t006 only ran a read-only `terraform plan`; no create was ever tried. The three refusals are confirmed in the provider's current code, so every Terraform user hits them on the first `apply`. w9/m474 grades the rest of the provider and should start after these fixes so it does not re-find them.
- **Decision state:** accepting each body is the proposal's **recommendation** under ADR008 (Render accepts them, per its own recordings), not a user decision. Relaxing bex's strict decoding of unknown fields in general was deliberately not proposed: it is an intentional design (ADR006:64, ADR049), and t003 accepts only the one field Render's client sends.
- **Render parity task:** included. The milestone changes REST create/PATCH for services, webhooks and Postgres; t006 confirms GraphQL and MCP.
