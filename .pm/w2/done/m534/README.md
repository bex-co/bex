# w2 · m534 — Pod-bound outbound HTTPS credential broker on ssh-gateway

**Worker:** worker2 **Goal:** Build the runtime data-plane credential broker inside `bex-ssh-gateway` and its internal `bex-api` lookup route so `/v1/sandboxes` Pods authenticate by source Pod IP alone, pass through non-credentialed TLS untouched, terminate TLS only for credentialed domains, enforce `networkPolicy` + SSRF protection, and inject `header`/`basic`/`cookie`/`query` secrets without ever exposing secret values to the sandbox. **Status:** done

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Add internal control-plane broker lookup endpoint on `bex-api` (`:8091`) returning a sandbox's `networkPolicy` and active group credentials with OpenBao secrets and short-TTL cache invalidation — **DONE** | 1h | `w2/m532` |
| t002 | Implement Sandbox Broker CA loader and bounded in-memory per-host short-lived leaf TLS certificate signer — **DONE** | 1h | t001 |
| t003 | Implement Render request-matching and secret-injection engine (`domain`, `methods`, `*`/`**` `paths`, `${SECRET}` template, `header`/`basic`/`cookie`/`query`, single-match non-combination) — **DONE** | 1h | t001 |
| t004 | Build `sshgateway/sandboxbroker` HTTP `CONNECT` proxy with Pod-IP auth, `networkPolicy` + resolve-once SSRF enforcement, pass-through TLS, selective MITM injection, and no redirect following — **DONE** | 1h30m | t002, t003 |
| t005 | Wire dedicated broker listener in `cmd/ssh-gateway`, concurrency/timeout limits, secret-free error/log scrubbing, and Prometheus metrics + Grafana panel — **DONE** | 45m | t004 |
| t006 | Add loopback end-to-end tests covering all 4 injection modes, pass-through TLS, `allow-list`/`deny-all`, SSRF/rebinding, 302 non-leakage, and live cutoff — **DONE** | 1h | t005 |
| t007 | Simplify — **DONE** | 15m | t006 |
| t008 | Test coverage — **DONE** | 30m | t006 |
| t009 | Closeout — **DONE** | 10m | t007, t008 |

## Definition of done

- `bex-ssh-gateway` serves the Pod-bound HTTPS credential broker on a dedicated cluster-internal port, identifying callers strictly by direct source Pod IP + immutable sandbox labels (`app.bex.co/regime=sandbox`, `bex.co/sandbox-id`, `bex.co/sandbox-group`, non-terminating, and `bex.co/agent-session` absent).
- For any `CONNECT host:443`: non-credentialed hosts pass through raw TLS unmodified; credentialed hosts terminate TLS with a short-lived Broker-CA leaf cert, inject `header`/`basic`/`cookie`/`query` only when `domain`, `methods`, and `paths` match, and forward over verified public-root TLS.
- `deny-all` is refused; `allow-list` refuses any host outside the sandbox's rules even if a group credential exists for it; SSRF attempts to loopback/private/link-local/metadata/cluster IPs are refused before dialing; HTTP 3xx responses return to the sandbox unfollowed; zero secret bytes appear in logs, error responses, or metrics.
- Credential disable, delete, value update, or version restore takes effect on new requests within the short cache TTL (≤ 5 s) without restarting sandboxes.

## Source + Goal linkage

- **Source:** `/pm-brainstorm one aspect of render parity for w2` (2026-11-18), materialized via `/pmx them all to w2`. Follows Render's OpenAPI credential matching/injection contract (`lego/backend/internal/api/openapi/render-public-api-1.json`) and reuses `ssh-gateway`'s Pod-bound proxy architecture (`lego/backend/internal/sshgateway/egressproxy/egressproxy.go`, `sessionpod.go`).
- **Goal linkage:** Render parity first ([ADR008 § Render parity first](../../../../docs/ADR008-vision.md#render-parity-first)) — delivers the core host-side secret injection mechanism of Render Sandbox credential brokering.
- **Expected outcome:** A self-contained, thoroughly tested host-side injection proxy inside `bex-ssh-gateway` ready for sandbox Pods to route through in `w2/m535`.
- **Why now:** Without the data-plane broker, `w2/m532`'s stored credentials are inert metadata; keeping the proxy separate from cluster/Cilium Pod wiring (`w2/m535`) isolates risk and keeps each milestone reviewable.
- **Render parity:** Omitted — internal gateway data plane and internal port-8091 broker lookup with no new public REST/GraphQL/MCP/UI wire schema; Render matching/injection semantics are verified by unit/loopback contract tests here and end-to-end on production in `w2/m536`.
- **Depends on:** `w2/m532`.
- **Brainstorm recommendations, not user decisions:** (1) `HTTPS_PROXY` HTTP `CONNECT` broker with selective MITM on credentialed domains only (uncredentialed domains stay pass-through TLS); (2) deterministic tie-breaking by lexicographical `credentialId` when overlapping rules match (Render's OpenAPI specifies non-combination without documenting tie-break order). Both are recorded in the `w2/m532` ADR with open sign-offs.
