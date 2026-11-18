# oidc-ai

A Python web service that calls the Anthropic Messages API and the OpenAI Responses API with **no model API key**: each provider trusts the workspace's Managed OIDC issuer, and the platform mounts a short-lived token for it, as on Render ([render.com/docs/oidc](https://render.com/docs/oidc); [ADR116 D10](../../docs/ADR116-managed-oidc.md#d10--anthropic-and-openai-w2m478)). It needs a Pro workspace or higher.

| Route | What it does |
| --- | --- |
| `GET /anthropic` | One Messages call through the zero-argument `anthropic.Anthropic()` client. 200 with the reply. |
| `GET /openai` | One Responses call through `openai.OpenAI(workload_identity=…)`. 200 with the reply. |
| `GET /healthz` | Always 200, so the service goes live before either provider is set up. |

A route answers 503 when a stored credential is set (`ANTHROPIC_API_KEY`, `ANTHROPIC_AUTH_TOKEN`, `ANTHROPIC_PROFILE`, `OPENAI_API_KEY`, `OPENAI_ADMIN_KEY`), because such a credential would outrank federation and a success would prove nothing. It also answers 503 when its provider is not configured, and 502 with the provider's error when the call fails.

## Set up the providers

Replace `<tea-…>` with your workspace id (the top of its Settings page). The issuer is `https://oidc.bex.co/<tea-…>`. On Render it is `https://oidc.render.com/<tea-…>`; nothing else changes.

**Anthropic** (Claude Console, an organization admin):

1. **Service accounts**: create a service account with the Developer role.
2. **Workload identity federation → Issuers**: connect a Custom OIDC provider with issuer URL `https://oidc.bex.co/<tea-…>` and JWKS source **OIDC Discovery**.
3. **Rules**: create a federation rule with subject pattern `workspace:<tea-…>:environment:<evm-…|default>:service:<srv-…|*>`, expected audience `api.anthropic.com`, and a token lifetime of at least 30 minutes.

**OpenAI** (organization settings, an organization admin):

1. **Security → Workload Identity Provider**: create one with OIDC issuer URL `https://oidc.bex.co/<tea-…>` and audience `api.openai.com`.
2. Create a mapping on it: `sub` = the same pattern as above, and a service account. Copy the service account's User ID from the **People** tab.

## Deploy

Deploy `render.yaml` as a Blueprint, or create a Python web service with build command `pip install -r requirements.txt` and start command `python main.py`. A Docker web service builds from the `Dockerfile` here. Set these variables, then redeploy:

| Variable | Value |
| --- | --- |
| `ANTHROPIC_FEDERATION_RULE_ID` | the federation rule's id. Setting it makes the platform mount the token and set `ANTHROPIC_IDENTITY_TOKEN_FILE`. |
| `ANTHROPIC_ORGANIZATION_ID` | the organization's id |
| `ANTHROPIC_SERVICE_ACCOUNT_ID` | the service account's id |
| `ANTHROPIC_WORKSPACE_ID` | the Anthropic workspace's id. Required when the rule spans several workspaces. |
| `OPENAI_IDENTITY_PROVIDER_ID` | the identity provider's id. Setting it makes the platform mount the token and set `OPENAI_IDENTITY_TOKEN_FILE`. |
| `OPENAI_SERVICE_ACCOUNT_ID` | the service account's User ID |
| `ANTHROPIC_MODEL`, `OPENAI_MODEL` | optional. The defaults are `claude-opus-5-5` and `gpt-5-mini`. |

Do not set `ANTHROPIC_IDENTITY_TOKEN_FILE` or `OPENAI_IDENTITY_TOKEN_FILE` yourself: the platform's value replaces yours. Then `curl https://<service>.onbex.co/anthropic` and `/openai`.

To test the Anthropic rule from the Console's **Test connection** tab, open a shell on the service (`bex ssh`) and run the script it gives you with `JWT=$(cat $ANTHROPIC_IDENTITY_TOKEN_FILE)`.

## Why `get_token` reads the file every time

The platform replaces each token every 20 minutes, and a token lives 60. The Anthropic SDK rereads `ANTHROPIC_IDENTITY_TOKEN_FILE` at every exchange. The OpenAI SDK calls `get_token` at every exchange, so `main.py` passes a function that reads the file. Render's own Python example passes `Path(TOKEN_PATH).read_text().strip`, which reads the file once, when the client is built, and fails its first exchange after the hour.
