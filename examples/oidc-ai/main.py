# oidc-ai: call the Anthropic Messages API and the OpenAI Responses API with
# Managed OIDC alone, no model API key anywhere (docs/ADR116-managed-oidc.md
# D10, w2/m478). On a Pro-or-higher workspace, setting ANTHROPIC_FEDERATION_RULE_ID
# or OPENAI_IDENTITY_PROVIDER_ID and redeploying makes the platform mount a
# token file and set ANTHROPIC_IDENTITY_TOKEN_FILE or OPENAI_IDENTITY_TOKEN_FILE,
# as on Render (render.com/docs/oidc). See README.md for the provider setup.
#
#   GET /anthropic  one Messages call; 200 with Claude's reply
#   GET /openai     one Responses call; 200 with the model's reply
#   GET /healthz    always 200, so the service goes live before either is set up
import functools
import os
from pathlib import Path

import anthropic
import openai
from flask import Flask, jsonify

app = Flask(__name__)

# A stored credential would outrank federation in the Anthropic SDK (an API
# key or auth token, even empty, and a named profile all win over the
# federation variables), so a success here would not prove federation.
STORED_CREDENTIALS = ("ANTHROPIC_API_KEY", "ANTHROPIC_AUTH_TOKEN", "ANTHROPIC_PROFILE", "OPENAI_API_KEY", "OPENAI_ADMIN_KEY")

def stored_credentials():
    return [name for name in STORED_CREDENTIALS if name in os.environ]


@functools.cache
def anthropic_client():
    # The zero-argument client reads ANTHROPIC_FEDERATION_RULE_ID,
    # ANTHROPIC_ORGANIZATION_ID, ANTHROPIC_SERVICE_ACCOUNT_ID,
    # ANTHROPIC_WORKSPACE_ID and ANTHROPIC_IDENTITY_TOKEN_FILE (anthropic >=
    # 0.98.0), exchanges the token for an access token, and rereads the file
    # whenever it exchanges again, so the platform's refresh reaches it.
    return anthropic.Anthropic()


@functools.cache
def openai_client():
    # The OpenAI SDK reads none of the variables itself (openai >= 2.37.0).
    # get_token must read the file on every call: the platform replaces it
    # every 20 minutes and a token lives 60, so a token read once when the
    # client is built would fail its first exchange after an hour.
    token_file = Path(os.environ["OPENAI_IDENTITY_TOKEN_FILE"])
    return openai.OpenAI(
        workload_identity={
            "identity_provider_id": os.environ["OPENAI_IDENTITY_PROVIDER_ID"],
            "service_account_id": os.environ["OPENAI_SERVICE_ACCOUNT_ID"],
            "provider": {"token_type": "jwt", "get_token": lambda: token_file.read_text().strip()},
        }
    )


def refuse(provider, needed):
    stored = stored_credentials()
    if stored:
        return {"provider": provider, "error": f"stored credentials are set ({', '.join(stored)}); unset them to prove federation"}
    missing = [name for name in needed if not os.environ.get(name)]
    if missing:
        return {"provider": provider, "error": f"not configured: {', '.join(missing)} unset (see README.md)"}
    return None


@app.get("/anthropic")
def ask_claude():
    refused = refuse("anthropic", ("ANTHROPIC_FEDERATION_RULE_ID", "ANTHROPIC_ORGANIZATION_ID", "ANTHROPIC_IDENTITY_TOKEN_FILE"))
    if refused:
        return jsonify(refused), 503
    try:
        message = anthropic_client().messages.create(
            model=os.environ.get("ANTHROPIC_MODEL", "claude-opus-5-5"),
            max_tokens=16000,
            output_config={"effort": "low"},
            messages=[{"role": "user", "content": "Reply with one short sentence greeting a service on bex."}],
        )
    except anthropic.APIError as err:
        return jsonify({"provider": "anthropic", "error": f"{type(err).__name__}: {err}"}), 502
    if message.stop_reason == "refusal":
        return jsonify({"provider": "anthropic", "error": "the model declined the request", "stop_reason": "refusal"}), 502
    text = "".join(block.text for block in message.content if block.type == "text")
    return jsonify({"provider": "anthropic", "auth": "workload identity federation", "model": message.model, "reply": text})


@app.get("/openai")
def ask_openai():
    refused = refuse("openai", ("OPENAI_IDENTITY_PROVIDER_ID", "OPENAI_SERVICE_ACCOUNT_ID", "OPENAI_IDENTITY_TOKEN_FILE"))
    if refused:
        return jsonify(refused), 503
    try:
        response = openai_client().responses.create(
            model=os.environ.get("OPENAI_MODEL", "gpt-5-mini"),
            input="Reply with one short sentence greeting a service on bex.",
        )
    except openai.OpenAIError as err:
        return jsonify({"provider": "openai", "error": f"{type(err).__name__}: {err}"}), 502
    return jsonify({"provider": "openai", "auth": "workload identity federation", "model": response.model, "reply": response.output_text})


@app.get("/healthz")
def health():
    return "ok\n"


if __name__ == "__main__":
    port = int(os.environ.get("PORT", 10000))
    print(f"oidc-ai listening on {port}")
    app.run(host="0.0.0.0", port=port)
