# w9 · m174 — Session-scoped secrets for cloud agents (Devin parity)

**Worker:** worker9 **Goal:** a customer can give an agent session the API keys it needs (e.g. for integration tests or third-party APIs) without putting them in the repo or prompt, and they never land in the sandbox in plaintext beyond the session. **Status:** in progress — t001–t007 done; t008 closes after the live check on production (secret env var used by the agent, value absent from transcript/API reads) and the composer skeleton check at desktop and narrow-mobile widths

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Design note in ADR047/ADR094: session secrets model — **DONE** | 45m | — |
| t002 | Store and attach secrets (API) — **DONE** | 90m | t001 |
| t003 | Deliver into the sandbox and redact — **DONE** | 90m | t002 |
| t004 | Dashboard: add secrets when starting a session — **DONE** | 60m | t002 |
| t005 | Render parity (Devin reference for agent surfaces) — **DONE** | 25m | t003, t004 |
| t006 | Simplify — **DONE** | 15m | t005 |
| t007 | Test coverage — **DONE** | 45m | t005, t006 |
| t008 | Closeout | 10m | t007 |

## Definition of done

- A session given a secret can use it (live check: agent reads an env var and calls a test endpoint), the value never appears in transcript, logs or any API read, and Devin-equivalent inputs (stored refs + inline) exist on REST/GraphQL/MCP/dashboard.

## Source + Goal linkage

- **Source:** user request 2026-11-05 ("把 w21 以及需要跟进这些问题移动到 w9 然后你统一跟进解决", `/pm them all`), from the ADR094/ADR095 recap next-steps list.
- **Goal linkage:** ADR008 pillar 5 cloud agents; Devin parity (secrets).
- **Expected outcome:** agents can run real tasks that need credentials (integration tests, third-party APIs).
- **Why now:** top Devin gap by customer impact in the 2026-11-02 gap list.
- **Parity:** included — agent surface; Devin reference.

## Live check (2026-11-06)

- **Live check attempt 2026-11-06 ~13:10 UTC: NOT RUN.** Production bex-api is still on the 92ff00293 pin (gitops `bab5d417e`). `GET /v1/agent-skills` and `GET /v1/agent-secrets` answer 404, and capabilities has no per-profile `modelKeyReady`. Every deploy run since then (379af6c08, 3310f6484, 8c498d33c) ended `success` with `build`/`deploy` skipped by supersession, so none of this work is live yet. Rerun after a real pin.

## Live check (2026-11-06, production 24a9203ee)

- Production pin 24a9203ee (gitops `d65dc2f2b`). An authenticated `GET /v1/agent-skills` and `GET /v1/agent-secrets` answered 200. QA user via CLI device login, workspace `bex` (QA user is an admin there). Every fixture was deleted and verified 404; the CLI grant was revoked (204, then 401) and the Kratos session logged out.
- **PASS — secret used, value never shown.** A workspace secret `QA_API_WS_SECRET` (PUT 200) and an inline `QA_SECRET`, each 31 random characters, were attached to the repo-less claude session `ags-dbmu0pa7qbhc739703q0`. The turn reported `ws_len=31` and `in_len=31`, and SHA-256 prefixes `bf760005352a`/`f607c224d25b`, matching the values the verifier computed locally. Neither raw value appeared in:
  - the transcript (2 pages, 5 parts);
  - the REST session read;
  - the GraphQL `agentSession` (which lists `secretNames` only);
  - the MCP `get_agent_session`;
  - the REST or MCP secret lists.
- The workspace secret was deleted (204) and the session deleted.
- **Not run:** the composer skeleton check at desktop and narrow-mobile widths (browser; another fork's scope).

## Browser check (2026-11-06, production 24a9203ee)

QA user in the `bex` workspace, Playwright at 1440×900 and 375×812. Fixtures used the `qa-ui-` prefix and were all deleted; the Kratos session was logged out.

- **PASS — Secrets card.** Adding `QA_UI_SECRET` and replacing its value (same name) both saved. The value never appeared in the DOM, before or after reload, and the name was listed. Delete removed it.
- **PASS — composer.** The Secrets popover listed the workspace secret, and choosing it showed "Secrets (1 attached)". Session `ags-dbmu4366kvbs738fl6qg` showed "1 secret" in its header, and the value appeared nowhere on the session page. The agent was asked to `printf %s "$QA_UI_SECRET" | wc -c`; its transcript did not show the count.
- **Skeleton (the Settings card): minor.** The pending Agent secrets card shows placeholder bars without its static "Agent secrets" heading and description. AGENTS.md asks a skeleton to keep the headings.

## Recheck (2026-11-06, production b8370c30c)

Production pin b8370c30c (gitops `ca9b90b8f`, deploy run 42238432288 success). Signed in as the QA user in the browser; API calls were made from it with the session cookie (REST, and MCP over `/mcp/cloud-agents`). The CLI device flow stopped at a Kratos reauthentication prompt, so no CLI grant was issued. Every fixture was deleted (DELETE 204, then GET 404), the Kratos session was logged out with this run's jar, and the run dir was removed.

- **Skeleton recheck (Settings → Agents, client navigation with `/graphql` held 8 s):**
  - **PASS:** the Agent secrets card skeleton now shows its static title and description (9 static texts shared by both states).
  - **Remaining delta:** with zero secrets the ready card is +20px at 375×812 and +4px at 1440×900 (skeleton 327/243px, ready 347/247px). The ready empty state is taller than the one-row skeleton. It is minor and depends on the data: the skeleton cannot know the count.
