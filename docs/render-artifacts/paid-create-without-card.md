# Render paid create without a payment method

**Evidence date:** 2026-07-31  
**Verdict:** best-evidenced public contract; no live cardless-account response body was available to capture

## What was checked

No expendable render.com workspace without payment information was available, so this investigation did not submit a chargeable create or modify a real paid resource. The evidence is therefore split explicitly between Render's published REST contract and previously observed dashboard behavior.

### REST

Render's public OpenAPI was checked through its official API documentation and the repository's complete pinned copy:

- upstream: `https://api-docs.render.com/openapi/render-public-api-1.json`
- pinned on 2026-07-20 at `lego/backend/internal/api/openapi/render-public-api-1.json`
- SHA-256: `2d27d5834d8bbc586e0aee62160cf996bb07f4be747112f15ec02c14fc11b315`

`POST /services` declares a `402` response referring to `#/components/responses/402PaymentRequired`. That component says, “You must enter payment information to perform this request.” Its JSON body uses the generic open `error` schema:

```json
{
  "id": "string (optional)",
  "message": "string (optional)"
}
```

The specification declares no response headers and does not provide an example body, a stable error id, or exact runtime message copy. Consequently, only the status, semantic description, and permitted `id`/`message` shape are evidenced—not a byte-for-byte render.com response.

The current public spec does **not** declare `402` on `POST /postgres` or `POST /key-value`. That absence is not evidence that cardless paid datastore creates succeed; it means their REST refusal status/body remains unverified. Render also declares the shared 402 on other paid operations such as dedicated-IP creation, which supports interpreting it as a general payment-required response.

### Dashboard

The authenticated Render workspace-plan capture in [workspace-plan-change.md](workspace-plan-change.md) observed a Payment Method section and the warning that paid services and compute require a card. No final paid submission was made. This establishes Render's just-in-time card requirement in the UI, but does not establish the exact paid-resource-create error presentation.

## Bex contract and accepted divergence

Bex matches the strongest public evidence: REST uses HTTP `402` and a Render-compatible body containing `id` and `message`. Its existing uniform REST error dialect also carries additive `error`, `code`, and `checkoutTool` fields:

```json
{
  "id": "payment_required",
  "error": "Payment information is required for paid plans. Call create_billing_checkout_session to add a payment method, then retry.",
  "message": "Payment information is required for paid plans. Call create_billing_checkout_session to add a payment method, then retry.",
  "code": "PAYMENT_REQUIRED",
  "params": {
    "checkoutTool": "create_billing_checkout_session"
  }
}
```

Those additive fields preserve bex's one REST error envelope and make the refusal actionable for the official CLI and agents. Bex applies the same payment gate to Service, Postgres, Key Value, and Blueprint paid intent even though Render's public spec documents 402 only for Service creation. This broader, uniform coverage is an intentional safety guarantee, not a claim that the undocumented Render datastore responses were captured.

## Local bex cross-surface verification

The current-tree API was built and run on 2026-07-31 with `BEX_REQUIRE_PAYMENT_METHOD=1`, a disposable Postgres 17 database, a local signature/introspection stub, a local Stripe API stub, and the CAPD app-cluster kubeconfig. No production credential or provider endpoint was used.

- REST `POST /v1/services` returned HTTP 402 and the exact envelope above.
- GraphQL `createService` returned `extensions: {"checkoutTool":"create_billing_checkout_session","code":"PAYMENT_REQUIRED"}` with the same message.
- MCP `create_web_service` returned `isError:true` and the same actionable text.
- The unmodified official Render CLI v2.21.0 printed `received response code 402: Payment information is required for paid plans. Call create_billing_checkout_session to add a payment method, then retry.` rather than a decoding error.
- The mounted dashboard integration test exercised GraphQL refusal → `BillingOnboardingView` dialog → readiness flip → the exact mutation's second invocation and success; its cancel case retained the single invocation and suppressed a generic error. This is component-level browser-DOM evidence, not a claim of a live render.com browser capture.

A correctly HMAC-signed `checkout.session.completed` fixture was accepted twice with HTTP 204. The handler re-read the completed setup session, Customer, Subscription, SetupIntent, and attached PaymentMethod from the local Stripe stub, bound both provider defaults, and monotonically stamped the Postgres marker. Retrying the byte-identical REST create then returned HTTP 201.

Emitter verification used a second cardless workspace with a sealed, in-horizon row. On gate-enabled startup the API logged one withheld workspace, the Stripe stub recorded `[]`, and the row remained `pending` with no `emitted_at`. After its signed Checkout completion and a fresh emitter pass, the stub recorded one Customer search, one Subscription lookup, and one meter event; the row became `emitted`. Starting the same tree with `BEX_REQUIRE_PAYMENT_METHOD` omitted let a third unstamped workspace's paid create return HTTP 201, confirming the env-off path. Both test Apps and all disposable processes/containers were removed afterward.

## Surfaces captured

| Surface | Evidence | Confidence |
| --- | --- | --- |
| REST service create | Official OpenAPI: declared 402 + generic error schema | High for status/schema; unknown exact live body/headers |
| REST Postgres / Key Value create | No 402 declared in current OpenAPI | Unknown live behavior |
| Dashboard | Previously observed payment-method requirement; no paid submit | High for requirement, unknown refusal presentation |
| GraphQL / MCP / official CLI | No cardless Render account exercised | No Render response capture; bex consistency is tested locally |

## Sources

- [Render API documentation](https://render.com/docs/api)
- [Render public OpenAPI](https://api-docs.render.com/openapi/render-public-api-1.json)
- [Bex's pinned-contract methodology](openapi-request-validation.md)
- [Authenticated Render workspace-plan capture](workspace-plan-change.md)
