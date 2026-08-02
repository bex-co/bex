# Agent sandbox image

This image is ADR047's in-sandbox execution environment. It contains git, Node.js, Python, Go, build tools, `claude-code-acp`, the gateway-backed Git credential helper, and the bex session driver. The driver owns one ACP agent over stdio and exposes port `8787`:

- `GET /stream` — AI SDK v6 UI-message SSE. Provider `raw` chunks are retained as standard `data-acp` parts so plans, diffs, and terminals survive the AI SDK's UI conversion.
- `GET /acp` (WebSocket upgrade) — newline-delimited ACP JSON-RPC pass-through for native clients.
- `POST /snapshot/scrub` — loopback-only model-credential scrub invoked by `/usr/local/bin/bex-pre-snapshot` before a rootfs snapshot.

The cluster's `sandbox-agent-driver-ingress` Cilium policy admits port `8787` only from the `bex-ssh-gateway` workload identity. The process intentionally binds the pod interface; loopback-only binding would make the gateway path impossible.

## Build and configure

Build from the `lego/` context:

```sh
docker build -f agent-image/Dockerfile -t bex-agent-sandbox:dev .
```

Agent choice is configuration, not image logic:

| variable | purpose |
| --- | --- |
| `BEX_AGENT_COMMAND` | ACP agent executable (default `claude-code-acp`) |
| `BEX_AGENT_ARGS` | JSON string array of arguments |
| `BEX_AGENT_CWD` | pre-cloned repository (default `/workspace`) |
| `BEX_AGENT_PROMPT` | starts one headless `streamText` turn when non-empty |
| `BEX_AGENT_EXISTING_SESSION_ID` | resume candidate, gated by an ACP `loadSession` probe |
| `BEX_AGENT_ENV_JSON` | non-secret string-to-string child environment |
| `BEX_AGENT_MODEL_API_KEY` | ephemeral OpenBao-sourced key consumed by the driver |
| `BEX_AGENT_MODEL_API_KEY_ENV` | agent-native key name (default `ANTHROPIC_API_KEY`) |
| `BEX_AGENT_SESSION_LOG` | local JSONL transcript path |
| `BEX_AGENT_STATUS_FILE` | machine-readable `running`/`succeeded`/`failed` status |
| `BEX_AGENT_TURN_TIMEOUT_MS` | hard headless-turn bound (default four hours) |
| `BEX_AGENT_SCRUB_ROOTS` | comma-separated persisted roots checked for the exact model credential before snapshot |
| `BEX_AGENT_EXIT_AFTER_TURN` | `1` exits after the one headless turn; otherwise listeners stay |

The `@mcpc-tech/acp-ai-provider` and AI SDK versions are exact pins in `driver/package.json`. The driver probes `initialize.agentCapabilities` before passing an existing session id because provider 0.2.9 otherwise calls `session/load` unconditionally.

## Credential and snapshot contracts

Git is configured system-wide with `credential.helper=bex` and `credential.useHttpPath=true`. `git-credential-bex get` calls the internal gateway for every Git operation and emits a one-hour, repository-narrowed GitHub installation token only over stdout to Git; `store` and `erase` are no-ops. There is no cache or token file. The session-create path stamps the binding labels returned by `agentsession.BindingLabels` and supplies the non-secret namespace, session, repository, branch, and credential-gateway values.

The model key is injected only as `BEX_AGENT_MODEL_API_KEY`. The driver removes the generic and agent-native names from its own environment, passes the native key only to the ACP child, and redacts it from driver logs. Immediately before an OpenSandbox rootfs snapshot, the lifecycle layer executes `/usr/local/bin/bex-pre-snapshot` through the signed sandbox-exec gateway. That hook calls the driver's loopback scrub endpoint, forgets the in-memory key, clears enumerated Git credential locations, and fails closed if either step fails. Resume starts a new process and refreshes credentials.

`credential-e2e-test.sh` exercises clone/fetch/push, one-hour token rotation, stopped-rootfs commit, resume, and post-resume push against a hermetic GitHub-compatible origin. `credential-scrub-test.sh` locks the enumerated Git hygiene contract. Driver vendor tests lock ACP spawn, raw chunks, session resume gating, listener behavior, and exact-key redaction.

## Live verification

The disposable production-equivalent sandbox verifier has an opt-in m37 leg. After the image and policy are deployed, an explicitly authorized run creates an `agent` template sandbox, executes a hermetic one-turn commit in its real gVisor worktree, round-trips the bundled ACP agent's `initialize`, and proves port `8787` admits the exact gateway workload identity while rejecting a label spoof and peer sandbox:

```sh
BEX_LIVE_VERIFY=1 BEX_VERIFY_AGENT_DRIVER=1 \
  BEX_API_URL=https://api.bex.co KUBECONFIG=/path/to/app.kubeconfig \
  bash scripts/verify-sandbox-isolation-live.sh
```

`BEX_VERIFY_AGENT_MODEL=1` is a separate paid-call gate. With `BEX_LIVE_AGENT_MODEL_API_KEY` loaded from the approved tenant OpenBao path, it runs the bundled agent against a disposable repo, requires a commit, and proves the key is absent from persisted writable roots. The key is sent to the pod over stdin and is never placed in an argument or file.
