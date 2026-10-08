# hello-python: a minimal Flask web service. MESSAGE is overridable via render.yaml
# envVars so a redeploy changes the response; PORT is injected by the platform.
import os
from flask import Flask

app = Flask(__name__)
message = os.environ.get("MESSAGE", "hello from bex")


@app.get("/")
def root():
    return message + "\n"


@app.get("/healthz")
def health():
    return "ok\n"


if __name__ == "__main__":
    port = int(os.environ.get("PORT", 3000))
    print(f'hello-python listening on {port}: "{message}"')
    app.run(host="0.0.0.0", port=port)
