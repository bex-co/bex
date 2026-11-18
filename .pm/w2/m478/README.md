# w2 · m478 — Services reach Anthropic and OpenAI without API keys through Managed OIDC, as on Render

**Worker:** worker2 **Goal:** A Pro-or-higher service that sets Render's Anthropic or OpenAI federation variables gets an audience-bound token at `ANTHROPIC_IDENTITY_TOKEN_FILE` / `OPENAI_IDENTITY_TOKEN_FILE`, so it calls the model APIs with no stored API key. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Add the Anthropic and OpenAI providers: triggers, audiences, file variables; several providers per service | 45m | w2/m477/t004 |
| t002 | Pin which variables trigger each provider, and what the official SDKs read | 45m | t001 |
| t003 | Example app and audience-binding check | 60m | t002 |
| t004 | Production acceptance (user gate: Claude Console and OpenAI organization setup) | 45m | t003 |
| t005 | Render parity | 20m | t004 |
| t006 | Simplify | 15m | t005 |
| t007 | Test coverage | 30m | t005 |
| t008 | Closeout | 10m | t006, t007 |

## Definition of done

- A Pro-workspace service with Render's Anthropic variables gets `ANTHROPIC_IDENTITY_TOKEN_FILE` (`aud` `api.anthropic.com`); with the OpenAI variables, `OPENAI_IDENTITY_TOKEN_FILE` (`aud` `api.openai.com`); a service with several providers gets one token per audience.
- An Anthropic-audience token is refused where the AWS audience is expected, and vice versa (in-cluster check).
- On production, `examples/oidc-ai` with no model API key in its env completes an Anthropic Messages call and an OpenAI call through federation, once the user's provider-side setup is in place.

## Source + Goal linkage

- **Source:** `/pm-brainstorm one aspect of render parity for w2` (2026-11-17); the user filed all five proposed milestones with `/pm them all to w2` the same day. That reopens the 2026-08-23 deferral recorded in [`.pm/FUTURE-MAYBE.md`](../../FUTURE-MAYBE.md) (from `w2/025`) under the Render-parity-first rule (AGENTS.md, user decisions 2026-10-17/2026-10-20; precedent: `w2/m252` absorbed a FUTURE-MAYBE item under the same rule). Ledger: [ADR018](../../../docs/ADR018-render-parity.md) row "Managed OIDC for AI providers" (✖ on REST/GraphQL/MCP/UI, the last all-✖ row that is not an anti-goal) and [ADR030](../../../docs/ADR030-pricing.md)'s pricing-parity table ("Open gap | OIDC authentication"). Render changelog 2026-07-24: [Managed OIDC now supports Anthropic and OpenAI](https://render.com/changelog/managed-oidc-now-supports-anthropic-and-openai).
- **Goal linkage:** Render parity first and AI-native hosting (ADR008): the ledger row is titled "Managed OIDC for AI providers".
- **Expected outcome:** AI apps deployed on bex call Anthropic and OpenAI without storing model API keys.
- **Why now:** It is the headline of Render's July change and the row's name; with w2/m477's mechanism in place it is mostly a provider-table extension plus proof.
- **Render parity:** Included: same env-var behavior on every surface and the same runtime variables as Render's page.
- **Depends on:** w2/m477 (delivery mechanism).
- **User gate:** t004 needs Claude Console and OpenAI organization admin setup; see the task.
