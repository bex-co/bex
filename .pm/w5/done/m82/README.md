# w5 · m82 — Agent-session Git proxy: forward gzip-encoded git bodies (clone/push of many-ref repos) + full E2E workflow verification

**Worker:** worker5 **Goal:** an agent session against a repo with many refs (e.g. `bex-co/bex-security`, 887 refs) clones, works, and delivers end to end — the gateway Git proxy no longer corrupts gzip-encoded git request bodies. **Status:** done (2026-08-30; shipped `3677e562`, pinned `0f84265a` → digest `ee3e9594…`, live E2E passed)

## t006 live E2E evidence (2026-08-30)

- Shipped `3677e562`, deploy run 33280196960 green, pin `0f84265a` (`33800901d0f4`), prod `bex-ssh-gateway` running pinned digest `ee3e9594…` (fresh pods verified before the run).
- Fresh production session **`ags-da9n7napkpos739mfsug`** — workspace `tea-d98210cbbpdc73dcrkvg`, repo `bex-co/bex-security` (887 refs, the exact repo whose clone deterministically 502'd as `ags-da9l9e5a801s739cb2ig`), branch `bex-agent/m82-gzip-proxy-e2e`, agent `claude`.
- **Clone succeeded in one attempt**: audit trail 00:06:16–00:06:22 shows every git exchange `MintCredential allowed` → `ProxyCredential allowed`, including the gzipped pack-fetch POST that previously died; only the known-benign startup-race denial at 00:06:12 (retried +4s, passed). Gateway logs carry **zero** `agent git proxy: … failure` lines.
- Turn 1 completed ~40s after dispatch; **delivery pushed** through the receive-pack proxy (exchanges 00:06:43–46 all `allowed`): `HELLO_M82.md` on the branch with the exact requested content.
- **Steer turn**: redispatched onto a fresh sandbox (re-exercised the clone a second time, clean), completed as `turns=2`, head `a8fa5670`, file appended with the steer line — both commits live on `refs/heads/bex-agent/m82-gzip-proxy-e2e`.
- **Replay**: `GET /v1/agent-sessions/{id}/transcript` returns 66 durable parts across both turns (the data the dashboard session page renders). Dashboard-browser screenshot leg was not possible — `QA_EMAIL`/`QA_PASSWORD` are empty in `.env` — so the session view + transcript were verified via the identical backing REST surface; caller was a temporary Hydra API-key client (`m82-e2e-verify`, developer-bound to the workspace) that was **fully revoked after the run** (FGA tuple deleted, `tenant_members` row deleted, Hydra client deleted).

## Tasks (in order)

| id   | title                                                                                   | est | depends_on           |
| ---- | --------------------------------------------------------------------------------------- | --- | -------------------- |
| t001 | Forward `Content-Encoding` on the upload-pack proxy hop — **DONE**                      | 30m | —                    |
| t002 | Gzip-aware receive-pack validation (decompress before `validateReceivePack`) — **DONE** | 45m | t001                 |
| t003 | Log + metric the gateway's silent upstream-502 paths — **DONE**                         | 30m | t001                 |
| t004 | Simplify — `/simplify` over the changed code — **DONE**                                 | 30m | t002, t003           |
| t005 | Test coverage — gzip clone/push proxy behavior + failure modes — **DONE**               | 45m | t004                 |
| t006 | Live E2E: entire agent-session workflow end to end on production — **DONE**             | 45m | t005 (shipped image) |
| t007 | Closeout — **DONE**                                                                     | 15m | t006                 |

## Definition of done

A fresh production agent session bound to `bex-co/bex-security` (or any repo whose clone negotiation body exceeds 1 KiB) completes the **entire** workflow: sandbox provisions, the driver's `git clone` through `bex-ssh-gateway:8082` succeeds (no `HTTP 502 … expected 'packfile'`), the agent runs its turn, the session branch is pushed through the proxy (delivery succeeds), and the dashboard session page shows the terminal `completed` state with a live transcript. Backend unit tests cover gzipped upload-pack forwarding, gzipped receive-pack validation (branch confinement still enforced), and the upstream-failure log lines. `make lint-backend` and `cd lego/backend && go test ./...` green.

## Source + Goal linkage

- **Source:** production failure `ags-da9l9e5a801s739cb2ig` (2026-08-29, dashboard.bex.co/agents/…) diagnosed same day: git gzips any smart-HTTP RPC body > 1 KiB (`remote-curl.c` `post_rpc`), and the gateway's forward allowlist (`lego/backend/internal/sshgateway/agentcred/agentcred.go:243` — only `Accept`, `Content-Type`, `Git-Protocol`) strips `Content-Encoding: gzip` while passing the compressed bytes; GitHub answers 400 (reproduced: identical body with the header → 200), which the gateway maps to the opaque 502 at `agentcred.go:257`. Latent since the allowlist landed (`e91d5be8`, 2026-08-14); `bex-co/bex-security` (887 refs → ~40 KiB negotiation body vs `bex-co/bex`'s 574 bytes) is simply the first repo to cross git's gzip threshold. Audit trail shows the deterministic per-attempt pattern: `info/refs` + `ls-refs` proxied `allowed`, the pack-fetch POST mints then dies upstream, ×10 retries.
- **Goal linkage:** pillar 5 (agent sessions) reliability — direct successor to w5/m80 (bounded, clearly-attributed terminal failures) and w5/m72 (production recovery incl. large-repo handling); ADR047 D2 is the Git proxy design.
- **Expected outcome:** clones and pushes through the proxy work regardless of repo ref count / body size; when the upstream hop does fail, the gateway logs an attributable reason instead of a silent 502. Agent branches accumulate on active repos, so every repo trends toward the failing regime — today's deterministic failure class is eliminated rather than raced.
- **Why now:** any agent session on `bex-co/bex-security` fails 100% deterministically today, and medium-size pushes (1 KiB–1 MiB body, git's gzip band below the chunked `http.postBuffer` threshold) are likely rejected 403 by `validateReceivePack` parsing raw gzip — a delivery-path landmine for every repo.
- **Render parity:** omitted — internal sandbox→gateway→GitHub mechanism; no REST/GraphQL/MCP/UI schema or behavior change.
