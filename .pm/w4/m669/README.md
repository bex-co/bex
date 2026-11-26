# w4 · m669 — An image pull failure says which of Render's four causes stopped it, in the event, its email and the deploy

**Worker:** worker4 **Goal:** a failed image pull's `image_pull_failed` event (REST, GraphQL, MCP and webhooks), its Image Pull Failed email and the deploy's failure reason name one of Render's four causes (image not found, authentication error, registry unavailable, rate limited) with Render's remedy, so a person or an agent can tell a wrong tag from an expired credential or a throttled registry. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](t001.md) | A pull the node's kubelet fails is worded as one of the four causes, as the pre-pull refusal already is | 1h | — |
| [t002](t002.md) | A registry that answered only 429 for the rollout budget fails as rate limited, with Render's three remedies | 40m | — |
| [t003](t003.md) | `image_pull_failed` and its email carry the deploy's cause instead of "The image could not be pulled" | 1h | t001, t002 |
| [t004](t004.md) | dev-4 and production: a missing tag and a refused credential each read their cause on every surface | 40m | t003 |
| [t005](t005.md) | Render parity | 25m | t004 |
| [t006](t006.md) | Simplify | 15m | t005 |
| [t007](t007.md) | Test coverage | 30m | t005, t006 |
| [t008](t008.md) | Closeout | 10m | t007 |

Total: **8 tasks, about 4h40m**.

## Definition of done

- A deploy of a nonexistent tag, of an image whose registry refuses the service's credential, and of an image whose registry answers only 429 each fails with a reason that names its cause and Render's remedy, whether bex's image metadata read or the node's kubelet hit the failure first.
- The deploy's `image_pull_failed` event carries the same cause in `details.message` on REST `GET /v1/services/{id}/events`, GraphQL and MCP, and in the webhook payload. No surface answers "The image could not be pulled" when the cause is known.
- The Image Pull Failed email names the cause and its remedy under its lead.
- Every reason is worded in all 8 dashboard languages through the shared deploy-reason templates (w4/566).
- On production, a nonexistent tag deployed to a first-party fixture service reads the not-found cause in its deploy and its event. ADR018's Service events row and `docs/render-artifacts/service-events.md` record what shipped.

## Source + Goal linkage

- **Source:** `/pm-brainstorm one aspect other than postgresql to w4`, 2026-11-26 (the aspect: deploying a prebuilt image, as [render.com/docs/deploying-an-image](https://render.com/docs/deploying-an-image) describes), item 1. Re-checked by hand at `origin/main` `28b4f18bbe` before filing: no open, blocked or done item on any workstream owns it.
- **Render:** the Image Pull Failed event's details require `message` and `imageURL` (`lego/backend/internal/api/openapi/render-public-api-1.json`). Its docs list four causes, each with a remedy: "Image not found (`404` error)" (verify the image URL), "Authentication error" (update the credential), "Registry unavailable" (wait and retry, or check the provider's status page) and "Rate limited (`429` error)" (wait for the reset, use a credential with a higher limit, or mirror the image). Render's exact event wording is not public; until a Render account can capture it (w10/blocked/002), the doc's headings and remedies are the reference.
- **bex today:**
  - bex's own metadata read already names three causes in the deploy's reason: `image_not_found`, `image_credential_refused`/`image_access_refused` (w4/566) and `image_registry_unreachable` (w4/692) in `lego/types/v1alpha1/deploy_reasons.go:42-51`, chosen by `imageRefusalError` (`lego/operator/internal/controller/image_network.go:825-845`).
  - A 429 is seen while waiting (`reasonImageRegistryRateLimited`, `image_network.go:693`), but a deploy that ends that way fails as "did not answer … for {minutes} minutes", not as rate limiting.
  - When the metadata read passes and the node's kubelet fails the pull, the reason is the raw `image pull is failing: <image>: <kubelet message>` (`app_controller.go:4549`, `:6893`; w8/m44).
  - The `image_pull_failed` fact drops the condition's message (`lego/backend/internal/store/reconciler.go:1740-1748`, `:1884-1891`), and every surface answers the constant `imagePullFailedMessage` "The image could not be pulled" (`lego/backend/internal/events/rest.go:273-275`, `:451`). The email has no detail line for it (`lego/backend/internal/notifications/alert_email_worker.go:270-271`, `:462-464`).
- **Goal linkage:** [ADR008 § Render parity first](../../../docs/ADR008-vision.md#render-parity-first), and AI-native operation: an agent reading the event or the webhook can act on the cause (fix the tag, rotate the credential, wait) without reading pod state.
- **Expected outcome:** the event, the webhook and the email say what went wrong and what to do, in Render's terms, instead of one sentence for every failure.
- **Why now:** Docker Hub throttling of bex's shared egress is real (w4/blocked/429), and today it reads exactly like a wrong tag in the event. w4/m671 reuses these causes for its availability check, so this goes first.
- **Render parity:** included: REST, GraphQL, MCP, webhooks, the email and the dashboard Events feed all change.
