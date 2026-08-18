# Agent sandbox image

This image is ADR047's in-sandbox execution environment. It contains git, Node.js, Python, Go, build tools, the pinned Claude Code, Codex, and Gemini ACP adapters, the gateway-backed Git proxy path, and the bex session driver. The driver owns one ACP agent over stdio and exposes port `8787`:

- `GET /stream` — AI SDK v6 UI-message SSE. Provider `raw` chunks are retained as standard `data-acp` parts so plans, diffs, and terminals survive the AI SDK's UI conversion.
- `POST /turn` — accepted only with the gateway's signed, single-use driver grant; raw ACP launch is intentionally not exposed.
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
| `BEX_AGENT_CWD` | working directory / repository checkout (default `/workspace`) |
| `BEX_AGENT_PROMPT` | starts one headless ACP turn when non-empty |
| `BEX_AGENT_DELIVER` | `1` runs the delivery step (ADR047 D4): setup-phase clone of the session branch, then commit + push after the turn; requires `BEX_AGENT_BRANCH` |
| `BEX_AGENT_BRANCH` | the `bex-agent/*` branch the driver commits + pushes |
| `BEX_AGENT_REPO_URL` | HTTPS clone URL used when the workspace is empty (setup phase) |
| `BEX_AGENT_BASE_BRANCH` | PR base branch (default: the origin default branch) |
| `BEX_AGENT_GIT_NAME` / `BEX_AGENT_GIT_EMAIL` | commit author identity (defaults `bex agent` / `agent@bex.co`) |
| `BEX_AGENT_EXISTING_SESSION_ID` | resume candidate, gated by an ACP `loadSession` probe |
| `BEX_AGENT_ENV_JSON` | non-secret string-to-string child environment |
| `BEX_AGENT_MODEL_API_KEY` | ephemeral OpenBao-sourced key consumed by the driver |
| `BEX_AGENT_MODEL_API_KEY_ENV` | agent-native key name (default `ANTHROPIC_API_KEY`) |
| `BEX_AGENT_SESSION_LOG` | local JSONL transcript path |
| `BEX_AGENT_STATUS_FILE` | machine-readable `running`/`succeeded`/`failed` status |
| `BEX_AGENT_TURN_TIMEOUT_MS` | hard headless-turn bound (default four hours) |
| `BEX_AGENT_SCRUB_ROOTS` | comma-separated persisted roots checked for the exact model credential before snapshot |
| `BEX_AGENT_EXIT_AFTER_TURN` | `1` exits after the one headless turn; otherwise listeners stay |

## Delivery (ADR047 D4)

When `BEX_AGENT_DELIVER=1`, the driver owns the completion path so delivery is deterministic and enforced outside the agent (the Copilot model). Before the turn it checks out the session branch (cloning `BEX_AGENT_REPO_URL` when the workspace is empty). After the turn it stages the working tree, commits with an author derived from the prompt, and pushes `BEX_AGENT_BRANCH` through the trusted Git proxy, then extracts a bounded, redacted evidence digest (command log + test-output tails + a trailing output window, all size-capped with explicit truncation marking) from the session log. All of this lands in the status file — `delivery{branch,baseBranch,headSha,pushed,commits,changedFiles}` and `evidence{commandLog,testOutput,outputTail,truncated}` — which bex-api's Completer reads through the gateway exec boundary to open the draft PR. A push failure fails the turn (status `failed`), never hangs. `delivery.test.ts` locks clone/commit/push, the no-change no-op, and the evidence bounds against a hermetic local origin.

The `@mcpc-tech/acp-ai-provider` and AI SDK versions are exact pins in `driver/package.json`. The driver probes `initialize.agentCapabilities` before passing an existing session id because provider 0.2.9 otherwise calls `session/load` unconditionally.

## Credential and snapshot contracts

`BEX_AGENT_REPO_URL` points at the gateway's Pod-bound Git smart-HTTP proxy. The gateway verifies the direct source Pod against immutable session/repository/branch labels, mints the repository-narrowed GitHub installation token internally, and injects it only on the TLS upstream hop. The sandbox never receives a raw token; receive-pack is parsed and rejected unless every update targets its exact `bex-agent/*` branch. The image contains no Git credential helper or credential cache.

Live turns are a second signed boundary. The driver receives only a derived Ed25519 public key and rejects `POST /turn` unless the gateway supplies a short-lived, session/action-bound, single-use grant after ticket redemption and fresh authorization. There is no raw `/acp` launcher route for same-Pod code to invoke.

The model key is injected only as `BEX_AGENT_MODEL_API_KEY`. The driver removes the generic and agent-native names from its own environment, passes the native key only to the ACP child, and redacts it from driver logs. Immediately before an OpenSandbox rootfs snapshot, the lifecycle layer executes `/usr/local/bin/bex-pre-snapshot` through the signed sandbox-exec gateway. That hook calls the driver's loopback scrub endpoint, forgets the in-memory key, clears enumerated Git credential locations, and fails closed if either step fails. Resume starts a new process and refreshes credentials.

The gateway's `internal/sshgateway/agentcred` tests exercise clone/fetch/push smart-HTTP mediation, exact-ref rejection, Pod binding, and the no-token-response invariant. `credential-scrub-test.sh` locks the enumerated Git hygiene contract. Driver vendor tests lock ACP spawn, raw chunks, session resume gating, listener behavior, signed turn grants, and exact-key redaction.

## Live verification

The disposable production-equivalent sandbox verifier has an opt-in m37 leg. After the image and policy are deployed, an explicitly authorized run creates an `agent` template sandbox, executes a hermetic one-turn commit in its real gVisor worktree, round-trips the bundled ACP agent's `initialize`, and proves port `8787` admits the exact gateway workload identity while rejecting a label spoof and peer sandbox:

```sh
BEX_LIVE_VERIFY=1 BEX_VERIFY_AGENT_DRIVER=1 \
  BEX_API_URL=https://api.bex.co KUBECONFIG=/path/to/app.kubeconfig \
  bash scripts/verify-sandbox-isolation-live.sh
```

`BEX_VERIFY_AGENT_MODEL=1` is a separate paid-call gate. With `BEX_LIVE_AGENT_MODEL_API_KEY` loaded from the approved tenant OpenBao path, it runs the bundled agent against a disposable repo, requires a commit, and proves the key is absent from persisted writable roots. The key is sent to the pod over stdin and is never placed in an argument or file.
