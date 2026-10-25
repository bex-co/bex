# Agent sandbox image

This image is ADR047's in-sandbox execution environment. It contains git, Node.js, Python, Go, build tools, the pinned Claude Code, Codex, and Gemini ACP adapters, the gateway-backed Git proxy path, and the bex session driver. The driver owns one ACP agent over stdio and exposes port `8787`:

- `GET /stream` — AI SDK v6 UI-message SSE. Provider `raw` chunks are retained as standard `data-acp` parts so plans, diffs, and terminals survive the AI SDK's UI conversion.
- `POST /turn` — accepted only with the gateway's signed, single-use driver grant; raw ACP launch is intentionally not exposed.

The cluster's `sandbox-agent-driver-ingress` Cilium policy admits port `8787` only from the `bex-ssh-gateway` workload identity. The process intentionally binds the pod interface; loopback-only binding would make the gateway path impossible.

## Build and configure

Build from the `lego/` context:

```sh
docker build -f agent-image/Dockerfile -t bex-agent-sandbox:dev .
```

Agent choice is configuration, not image logic:

| variable | purpose |
| --- | --- |
| `BEX_AGENT_COMMAND` | ACP agent executable (default `claude-agent-acp`) |
| `BEX_AGENT_ARGS` | JSON string array of arguments |
| `BEX_AGENT_CWD` | working directory / repository checkout (default `/workspace`) |
| `BEX_AGENT_PROMPT` | starts one headless ACP turn when non-empty |
| `BEX_AGENT_DELIVER` | `1` marks a repo session (ADR090): setup-phase clone of the session branch, the `bex` MCP PR tool, and the post-turn delivery check; requires `BEX_AGENT_BRANCH` |
| `BEX_AGENT_BRANCH` | the bound `bex-agent/*` branch the agent pushes |
| `BEX_AGENT_REPO_URL` | HTTPS clone URL used when the workspace is empty (setup phase) |
| `BEX_AGENT_BASE_BRANCH` | PR base branch (default: the origin default branch) |
| `BEX_AGENT_REPOSITORY` | `owner/name` of the bound repository (named in the overlay) |
| `BEX_AGENT_GIT_NAME` / `BEX_AGENT_GIT_EMAIL` | repo-local author identity for the agent's commits (defaults `bex agent` / `agent@bex.co`) |
| `BEX_AGENT_EXISTING_SESSION_ID` | resume candidate, gated by an ACP `loadSession` probe |
| `BEX_AGENT_ENV_JSON` | non-secret string-to-string child environment |
| `BEX_AGENT_SESSION_ID` | session id; required |
| `BEX_AGENT_MODEL_PROXY_URL` | gateway model-proxy URL; required (no direct-to-vendor mode) |
| `BEX_AGENT_MODEL_API_KEY` | must be exactly the session placeholder `bex-model-proxy-placeholder-<session id>`; the driver refuses anything else |
| `BEX_AGENT_SESSION_LOG` | local JSONL transcript path |
| `BEX_AGENT_STATUS_FILE` | machine-readable `running`/`succeeded`/`failed` status; the `bex` MCP tool records the PR in `pull-request.json` beside it |
| `BEX_AGENT_TURN_TIMEOUT_MS` | hard headless-turn bound (default four hours) |
| `BEX_AGENT_EXIT_AFTER_TURN` | `1` exits after the one headless turn; otherwise listeners stay |

## Agent-owned delivery (ADR090)

The agent delivers its own work; the driver never commits or pushes. During setup the driver checks out the session branch (cloning `BEX_AGENT_REPO_URL` when the workspace is empty) and writes repo-local `user.name`/`user.email`, `push.default=current` and the branch upstream, so a plain `git commit` and `git push` land on the bound branch. Repo hooks run for the agent as they would for a developer; only the driver's own setup and read-only Git calls are hardened.

Every session carries the bex platform overlay (`driver/src/overlay.ts`): where the agent runs, what it can use, and that the deliverable is a draft PR. It travels through ACP `systemPrompt` append when the adapter advertises `sessionCapabilities.systemPrompt` (claude-agent-acp), as Codex `developer_instructions` via `CODEX_CONFIG` for codex-acp, and otherwise as a first-prompt preamble that is never published to the stream. An `invalid_params` reply fails the turn with a named reason. Claude sessions also disable Claude Code's built-in git instructions and attribution.

Repo sessions register the `bex` stdio MCP server (`dist/mcp-bex.js`) on `session/new` and `session/load`. Its single tool, `open_or_update_draft_pr({title, body})`, posts to the gateway's `pull-request` verb at `BEX_AGENT_REPO_URL` + `/pull-request`; head, base and repository are fixed by that path, and the gateway authenticates the source Pod. There is no GitHub token and no `gh` in the image.

When the agent ends its turn the driver reads Git state (dirty tree, local commits not on the remote branch, remote head, recorded PR). If work is undelivered it sends one follow-up prompt on the same ACP session, then re-reads the facts and writes them to the status file as `git{dirty,unpushedCommits,remoteHead,baseBranch,prRecorded,prUrl,prNumber}`.

The official `@agentclientprotocol/sdk` and AI SDK versions are exact pins in `driver/package.json`; each public ACP adapter is pinned and build-verified in the image Dockerfile. The driver probes `initialize.agentCapabilities` before calling optional `session/load`. For the pinned Codex adapter it also binds the OpenAI provider through ACP `providers/set` before `session/new`; `OPENAI_BASE_URL` alone can leave Codex on its built-in public origin. The binding carries only the session placeholder and the exact internal model-proxy URL. The driver negotiates Codex's typed session-failure extension and turns a terminal provider/transport record into the persisted failed verdict instead of accepting its `end_turn` wrapper as a false-green no-op.

## Credential and sandbox-lifetime contracts

`BEX_AGENT_REPO_URL` points at the gateway's Pod-bound Git smart-HTTP proxy. The gateway verifies the direct source Pod against immutable session/repository/branch labels, mints the repository-narrowed GitHub installation token internally, and injects it only on the TLS upstream hop. The sandbox never receives a raw token; receive-pack is parsed and rejected unless every update targets its exact `bex-agent/*` branch. The image contains no Git credential helper or credential cache.

Live turns are a second signed boundary. The driver receives only a derived Ed25519 public key and rejects `POST /turn` unless the gateway supplies a short-lived, session/action-bound, single-use grant after ticket redemption and fresh authorization. There is no raw `/acp` launcher route for same-Pod code to invoke.

The sandbox never holds a real model key. `BEX_AGENT_MODEL_API_KEY` is only the session-bound placeholder; the driver hands it to the ACP child under the adapter's native name next to the gateway model-proxy base URL, and the gateway swaps in the real key on its upstream hop (ADR062). There is therefore no credential scrub, Git-object scan or pre-snapshot hook. Nothing in the sandbox outlives it: there is no workspace snapshot or restore (ADR059 hibernation was retired on 2026-10-25). An idle session's sandbox is terminated after `BEX_AGENT_SANDBOX_IDLE_TTL` (2h), and the next message runs in a fresh sandbox that re-clones the session branch and is primed from the durable transcript (ADR047 D3).

The gateway's `internal/sshgateway/agentcred` tests exercise clone/fetch/push smart-HTTP mediation, exact-ref rejection, Pod binding, and the no-token-response invariant. Driver tests lock ACP spawn, typed chunks, session resume gating, listener behavior, signed turn grants, the overlay channels, the `bex` MCP tool, the agent Git environment and the post-turn delivery check.

## Live verification

The disposable production-equivalent sandbox verifier has an opt-in m37 leg. After the image and policy are deployed, an explicitly authorized run creates an `agent` template sandbox, executes a hermetic one-turn commit in its real gVisor worktree, round-trips the bundled ACP agent's `initialize`, and proves port `8787` admits the exact gateway workload identity while rejecting a label spoof and peer sandbox:

```sh
BEX_LIVE_VERIFY=1 BEX_VERIFY_AGENT_DRIVER=1 \
  BEX_API_URL=https://api.bex.co KUBECONFIG=/path/to/app.kubeconfig \
  bash scripts/verify-sandbox-isolation-live.sh
```

The same leg proves the image's driver refuses a standalone real-key launch. A paid model turn runs through a real agent session on the gateway model proxy (`scripts/agent-session-verify.sh`).
