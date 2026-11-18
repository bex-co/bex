# w2 · m478 — Services reach Anthropic and OpenAI without API keys through Managed OIDC, as on Render

**Worker:** worker2 **Goal:** A Pro-or-higher service that sets Render's Anthropic or OpenAI federation variables gets an audience-bound token at `ANTHROPIC_IDENTITY_TOKEN_FILE` / `OPENAI_IDENTITY_TOKEN_FILE`, so it calls the model APIs with no stored API key. **Status:** blocked (t001–t003, t006, t007 done; t004 needs the user's Claude Console and OpenAI organization setup and a production pin carrying this work, then t005, t008)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Add the Anthropic and OpenAI providers: triggers, audiences, file variables; several providers per service — **DONE** | 45m | w2/m477/t004 |
| t002 | Pin which variables trigger each provider, and what the official SDKs read — **DONE** | 45m | t001 |
| t003 | Example app and audience-binding check — **DONE** | 60m | t002 |
| t004 | Production acceptance (user gate: Claude Console and OpenAI organization setup) | 45m | t003 |
| t005 | Render parity | 20m | t004 |
| t006 | Simplify — **DONE** | 15m | t005 |
| t007 | Test coverage — **DONE** | 30m | t005 |
| t008 | Closeout | 10m | t006, t007 |

## Definition of done

- A Pro-workspace service with Render's Anthropic variables gets `ANTHROPIC_IDENTITY_TOKEN_FILE` (`aud` `api.anthropic.com`); with the OpenAI variables, `OPENAI_IDENTITY_TOKEN_FILE` (`aud` `api.openai.com`); a service with several providers gets one token per audience.
- An Anthropic-audience token is refused where the AWS audience is expected, and vice versa (in-cluster check).
- On production, `examples/oidc-ai` with no model API key in its env completes an Anthropic Messages call and an OpenAI call through federation, once the user's provider-side setup is in place.

## Source + Goal linkage

- **Source:** `/pm-brainstorm one aspect of render parity for w2` (2026-11-17); the user filed all five proposed milestones with `/pm them all to w2` the same day. That reopens the 2026-08-23 deferral recorded in [`.pm/FUTURE-MAYBE.md`](../../../FUTURE-MAYBE.md) (from `w2/025`) under the Render-parity-first rule (AGENTS.md, user decisions 2026-10-17/2026-10-20; precedent: `w2/m252` absorbed a FUTURE-MAYBE item under the same rule). Ledger: [ADR018](../../../../docs/ADR018-render-parity.md) row "Managed OIDC for AI providers" (✖ on REST/GraphQL/MCP/UI, the last all-✖ row that is not an anti-goal) and [ADR030](../../../../docs/ADR030-pricing.md)'s pricing-parity table ("Open gap | OIDC authentication"). Render changelog 2026-07-24: [Managed OIDC now supports Anthropic and OpenAI](https://render.com/changelog/managed-oidc-now-supports-anthropic-and-openai).
- **Goal linkage:** Render parity first and AI-native hosting (ADR008): the ledger row is titled "Managed OIDC for AI providers".
- **Expected outcome:** AI apps deployed on bex call Anthropic and OpenAI without storing model API keys.
- **Why now:** It is the headline of Render's July change and the row's name; with w2/m477's mechanism in place it is mostly a provider-table extension plus proof.
- **Render parity:** Included: same env-var behavior on every surface and the same runtime variables as Render's page.
- **Depends on:** w2/m477 (delivery mechanism).
- **User gate:** t004 needs Claude Console and OpenAI organization admin setup; see the task.

## Park 2026-11-18

- **Shipped (`5bbc7f938`, `e4a3b1fb4`).**
  - The Anthropic and OpenAI providers ([ADR116 D10](../../../../docs/ADR116-managed-oidc.md#d10--anthropic-and-openai-w2m478)). `ANTHROPIC_FEDERATION_RULE_ID` mounts `/var/lib/render/oidc/anthropic-token` named by `ANTHROPIC_IDENTITY_TOKEN_FILE` (`aud` `api.anthropic.com`). `OPENAI_IDENTITY_PROVIDER_ID` mounts `openai-token` named by `OPENAI_IDENTITY_TOKEN_FILE` (`aud` `api.openai.com`). A service gets one token per provider, each for its own audience.
  - `examples/oidc-ai`, and `scripts/oidc-audience-verify.sh` with its self-test.
- **Done:** t001–t003, t006, t007. On dev-2, a service with the AWS, Anthropic and OpenAI triggers got three tokens, and each was refused where another audience was expected ([t003](done/t003.md)). The example built and ran there with both files mounted, and both SDKs reached their provider's token exchange.
- **Gate (user):** the Claude Console federation setup and the OpenAI Workload Identity Provider for `https://oidc.bex.co/tea-daif693dqjvc73e7as3g`, with their ids passed to the worker ([t004](t004.md) lists each step). Both need an organization admin; bex holds no credential for either.
- **Gate (clears by itself):** a production pin carrying `5bbc7f938` and w2/m476's `68b45459b`. Check that `curl -sS https://oidc.bex.co/tea-daif693dqjvc73e7as3g/.well-known/openid-configuration` answers 200.
- **Then:** t004's production calls, with their replies in this README. Next, t005's production walk of the GraphQL, MCP and env-group writes, which dev-2 cannot make without OpenBao. Last, t008.
