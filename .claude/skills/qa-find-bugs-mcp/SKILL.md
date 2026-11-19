---
name: qa-find-bugs-mcp
description: >-
  Hunt live hosting bugs through bex's MCP server (https://api.bex.co/mcp and /mcp/cloud-agents) as a real MCP client against the pinned Render MCP contract, compare every answer with REST and GraphQL controls, trace failures into the MCP adapter or the shared service, and file researched, non-duplicate fixes through /pm. Use when asked to QA the MCP server, test the agent endpoint, or find hosting bugs from MCP journeys. Ships the filing only when explicitly authorized.
---

# Live MCP QA → contract diagnosis → researched PM filing

Drive the production MCP endpoint (`https://api.bex.co/mcp`) the way an agent does: JSON-RPC over streamable HTTP with a bearer credential. Apply the customer-journey discipline of `../qa-find-bugs-cli/SKILL.md`; this skill restates what differs for MCP and points there for the shared rules. Find and research bugs, then schedule fixes. Implementing fixes is separate work.

## Contract and scope

**Render's official MCP server, at the pinned commit, is the tool and wire-contract oracle.** bex mirrors Render's tool names, argument names, argument types, required sets, annotations and answers; bex-only tools are documented extensions. Read these before probing:

- `lego/backend/internal/api/openapi/render-mcp-tools.json`: the pinned upstream `tools/list` (names, args, types, required, annotations) and its `pin.note` history. `render-mcp-results.json` beside it pins what each shared tool answers.
- `lego/backend/internal/api/mcp_parity.go`: `mcpAcceptedDivergences` (each deliberate departure and its reason) and `mcpKnownUpstreamOnly` (upstream tools bex does not ship: Workflows, a recorded non-goal, and session selection, replaced by a per-call `workspaceId`).
- `docs/ADR006-bex-api.md` § MCP ("Transports & auth"), `docs/ADR025-connect-an-agent.md` (the customer's connect recipe, OAuth discovery, API-key bearer), `docs/ADR094-coding-agent-mcp.md` (the `/mcp/cloud-agents` toolset), `docs/ADR018-render-parity.md` (MCP column), `.pm/DO_NOT_DO.md`.

**When the correct behavior is unclear, learn it from Render before deciding.** Read the upstream server's source at the pin's `source.commit` (`github.com/render-oss/render-mcp-server`), Render's MCP docs (`https://render.com/docs/mcp-server`) and the REST endpoint the upstream tool calls (`render-public-api-1.json` beside the pin). Never call Render's hosted server or use a Render credential. For `/mcp/cloud-agents` and every coding-agent tool, the reference product is **Devin**, as ADR094 says. Render parity is the top product rule (`AGENTS.md`): a divergence that ADR018 does not record with the user's sign-off is a finding, even when a bex ADR chose it.

Arguments:

- `wN`: target workstream, default `w6`.
- Journey names from the table in § 2: restrict coverage. The default is the whole sweep.
- `DRY_RUN=1`: hunt and report, no board writes or ship. Disposable QA fixtures are still allowed.
- Honor explicit read-only or audit-only requests: skip creates and mutations and report the coverage gaps.

Repeated sweeps follow `../qa-find-bugs-cli/SKILL.md` § "Autonomous execution and repeated sweeps": choose the next useful journey yourself, a fresh fixture nonce each sweep, a sanitized checkpoint of owned IDs, human pace, no duplicate filings to keep busy.

## 0 — Preflight

Sync and record exactly as `../qa-find-bugs-cli/SKILL.md` § 0 does (pull `main` with a scoped stash, record branch, HEAD and `git status`). Also record:

- the deployed pin: the newest `chore(deploy): pin platform images to <sha>` commit on `origin/main` (`git log origin/main --grep 'pin platform images' -n1`). A fix on `main` that the pin does not contain (`git merge-base --is-ancestor <fix> <pin>`) is deploy lag, not a new bug;
- `initialize`'s `serverInfo` and negotiated `protocolVersion`;
- the credential kind (run-minted API key, OAuth token, anonymous) and its permissions. Never the value.

Every request goes through the helper, so the bearer never reaches argv, a tool call or a transcript:

```sh
python3 scripts/qa-mcp.py [--url URL] [--key-file FILE | --anonymous] [--timeout S] <command>
```

Commands: `initialize`, `tools-list`, `resources-list`, `prompts-list` (each follows `nextCursor` and names duplicates), `call TOOL [json-args]`, `rpc METHOD [json-params]`, `notify METHOD`, `raw --method M [--accept A] [--content-type T] [--body TEXT] [--no-protocol-header]` for transport checks, `rest METHOD PATH [json]` and `graphql QUERY [json-vars]` for same-credential controls on the API origin, `get-url URL` for discovery documents, and `mint-key` / `revoke-key` (§ 1). It prints status, elapsed time, an allowlist of headers and the parsed JSON-RPC messages of a JSON or SSE answer. It redacts the key, any `bex_…`/`ory_…` token and the session cookie. It exits `0` whatever the HTTP status, `2` for usage or credential problems and `3` for a network failure or a timeout (a hang is a result). Work in a private 0700 directory under the session scratchpad; key files there are 0600.

## 1 — Credentials and workspace

The customer paths are an API key sent as the bearer (`bex_<id>_<secret>`, as a Render API key is sent) and an OAuth token from the discovery flow (ADR025). Use a run-minted key for the hunt:

1. `bash scripts/qa-login.sh <private dir>/state.json` signs the QA user in from `QA_EMAIL`/`QA_PASSWORD` in `.env` or `muse.env`, without reading either file. Keep its `logout <jar>` line: that jar is the session.
2. Choose the workspace by the order in `../qa-find-bugs-cli/SKILL.md` § 1 step 4 (earlier sweeps used `bex-canary`). Read the candidates with the session, never by guessing an id.
3. `python3 scripts/qa-mcp.py mint-key --jar <jar> --owner-id <tea-…> --out <private dir>/key.json` mints an all-permissions key named `qa-<yyyymmdd>-<nonce>-mcp`. For the boundary journey, mint a second, read-only key with `--permissions '<grant>'`, built from the catalog (`rest GET '/v1/api-keys/permissions?ownerId=<tea-…>'`).
4. When the hunt ends, or the loop stops, revoke every key this run minted: `python3 scripts/qa-mcp.py revoke-key --jar <jar> --key-file <file>`. It deletes the file only after a `204`, so a failed revoke keeps the handle. Then `bash scripts/qa-login.sh --logout <jar>` and remove that jar and state file, never another run's.

If sign-in or minting fails, finish the anonymous transport and discovery journeys, mark every authenticated journey unverified and report the blocker. Missing credentials belong in `.env`, never chat. Never use `BEX_BOOTSTRAP_CLIENT_SECRET`, `BEX_CANARY_API_KEY`, admin or kubeconfig credentials to stand in for the QA user. The production kubeconfig (`scripts/fetch-app-kubeconfig.sh`) is for read-only diagnosis of a reproduced failure, not for exercising tools.

## 2 — Exercise whole MCP journeys

The fixture rules of `../qa-find-bugs-cli/SKILL.md` § 2 apply unchanged: `qa-<yyyymmdd>-<nonce>-<fixture>` names, a persisted intent before each create, free plans only, pre-existing resources immutable, delete by recorded id. Render's MCP has no delete tools, so cleanup goes through the helper's `rest DELETE` with the same key (or the CLI); poll until the detail read is `404`.

| Journey | What to prove |
| --- | --- |
| Transport | Anonymous `POST` answers `401` with `WWW-Authenticate: Bearer resource_metadata="…"`; that document (`get-url`) answers `200` naming an authorization server whose metadata also resolves (RFC 9728/8414). Authenticated `GET` answers `405` (stateless transport; ADR006) without hanging. A wrong `Accept` or content type, a malformed body, a batch and an unknown method fail as JSON-RPC errors before any tool runs; a notification answers `202`. A revoked key answers `401` on the next request. |
| Discovery | `initialize` negotiates the protocol versions the SDK supports and returns `serverInfo`. `tools/list` has unique names, every tool a description and `inputSchema`. Every upstream tool in the pin is served with the same argument names, types, required set and read-only/destructive hints, or is listed in `mcpKnownUpstreamOnly`; a served tool that drifts from the pin and is not in `mcpAcceptedDivergences` is a finding. `/mcp/cloud-agents` lists exactly its ADR094 toolset and refuses any other tool. |
| Workspace binding | `list_workspaces` names the key's workspace. Each tool honors an explicit `workspaceId`; `ownerId` is refused with the hint to use `workspaceId`; a workspace the key does not belong to is refused without revealing whether it exists. Compare with how upstream resolves the workspace before calling a difference a bug. |
| Boundaries | The read-only key reads, and every write tool returns a tool error naming the permission, never a success or an internal error. Unknown and other-workspace ids answer not-found the same way (no existence leak). The same calls through `rest` and `graphql` with the same key agree. |
| Reads | `list_services`, `get_service`, `list_deploys`, `get_deploy`, `list_logs`, `list_log_label_values`, `get_metrics`, `list_events`, `list_postgres_instances`, `get_postgres`, `list_key_value`, `get_key_value`: fields, ids, timestamps, pagination and filters match REST `/v1/…` on the same key, and their shape matches `render-mcp-results.json`. Empty and missing cases are distinct, never an empty success. |
| Writes | `create_web_service` (free, an image or a public repo), `create_static_site`, `create_cron_job`, `update_environment_variables`, `trigger_deploy`, `run_cron_job` / `cancel_cron_job_run`, and free `create_postgres` / `create_key_value` where capacity allows. Each create lands in the named workspace and appears in REST. A deploy reaches a terminal state, and the service answers on its URL with a harmless marker. Duplicate names, bad plans and bad regions fail with Render's message shape. |
| Blueprints | `create_blueprint` plans without applying, then `plan_blueprint`, `get_blueprint`, `approve_blueprint` and `update_blueprint` on a fixture `render.yaml` of free resources only, compared with the REST Blueprint routes. |
| SQL | `query_render_postgres` on an owned free database answers rows keyed by column, refuses any write statement (read-only transaction), and handles truncation and timeouts as the pin and ADR018 say. |
| Envelope | A tool failure is a result with `isError: true` and a readable message, not a JSON-RPC protocol error, and the reverse for protocol faults. A missing required argument or a wrong type names the argument and never echoes its value (secrets may sit in the wrong field). Structured content and text agree. |
| Limits | At human pace, a few extra calls show the rate-limit answer (`429`, `retry-after`) and its recovery; never a flood, never a write-budget probe on production. |
| Cloud agents | Read-only by default: `get_agent_session_capabilities`, `list_agent_sessions`, `list_workspaces`, `list_repos`, judged against Devin's behavior. Creating a session spends model credit; do it only when the user asks. |
| Clients | Only when the user asks (billed): a real Claude Code session added with `claude mcp add --transport http … --header "Authorization: Bearer ${…}"` lists the same tools and completes a read journey. Record the client version. |
| Cleanup | Delete by recorded id in dependency order; poll for absence in detail and list reads, and revoke this run's keys. |

Mark a surface the plan or capacity does not allow as uncovered, not failed. REST and GraphQL calls diagnose an MCP failure but do not count as MCP coverage.

## 3 — Reproduce and locate the failing boundary

Reproduce each candidate in a fresh request with the same credential kind and a working control (another tool, the REST route, a second fixture). Retry throttled calls after 30–60 s; a sweep-induced `429` is not a finding. `api.bex.co` sits behind Cloudflare: a Cloudflare page is an edge result, not an application error, and the edge decodes gzip for a client that did not ask for it.

Trace **tool call → transport and auth gate → MCP middleware → tool adapter → shared service → answer**:

- mount, middleware and argument-value redaction: `lego/backend/internal/api/server.go` (`/mcp`, `buildMCPServerFor`, `mcpToolsetHTTPHandler`);
- workspace binding: `internal/api/mcp_workspace.go`; scope gate: `requireMCPScope` in `internal/api/scope_matrix.go`; toolsets: `mcpToolsets`;
- auth: `internal/api/auth.go` and `auth_apikey.go`; API keys: `internal/apikeys`;
- tool adapters: `internal/<feature>/mcp.go` and `internal/mcputil`; each calls the same `Service` method as REST and GraphQL (ADR006), so a wrong answer on all three surfaces belongs to the service, and one on MCP alone to the adapter;
- the parity guard tests that should have caught it: `TestMCPParity*`, `TestMCPArgumentTypesMatchUpstream`, `TestMCPAnnotationsMatchUpstream`, `TestRenderMCPResultsCoverEverySharedTool`, `TestCrossWorkspaceMCPMatrix`, `TestMCPArgumentErrorsNeverEchoTheValue`.

Read the upstream tool at the pinned commit before naming the target behavior: which REST call it makes, which arguments it requires, what it returns on an empty or failed call. Keep the exact JSON-RPC body, status, relevant headers and the full redacted answer; replay the same request without repairing it first.

## 4 — Research, record, dedupe, file and ship

Research each finding as `../qa-find-bugs-cli/SKILL.md` § 4 does, and write one record per bug in its template with these fields changed:

- Versions: deployed pin, `serverInfo`, protocol version, the Render MCP pin commit;
- Context: date, endpoint (`/mcp` or `/mcp/cloud-agents`), workspace, credential kind and permissions;
- Repro: the exact `qa-mcp.py` commands with file placeholders;
- Expected / actual: the pinned upstream contract or Render's documented behavior, with the source you read;
- Fix: the target answer on MCP and on every REST/GraphQL/UI surface that shares the service.

Dedupe, filing and shipping follow `../qa-find-bugs-cli/SKILL.md` § 5: search `.pm/` open and done, `.pm/DO_NOT_DO.md`, ADR018 and `git log -S`; file only through `/pm` into the target workstream (default `w6`); a deploy-lag result is reported, not filed. **Commit or push only with an explicit `/ship`/`$ship`**, or when a `/loopx` routine runs this skill, which is that authorization; ship only this hunt's filing.

## 5 — Cleanup and report

Cleanup follows `../qa-find-bugs-cli/SKILL.md` § 6. Delete every owned fixture by id and confirm `404`, revoke every key this run minted, log out this run's Kratos session and remove its jar, state file and key files. Report the journeys exercised and skipped with reasons, findings by severity with root cause or stated uncertainty, ownership (adapter, service, auth, edge, upstream), filing locations, duplicates and deploy lag, and cleanup status, with any surviving fixture ids or unrevoked key ids listed first.
