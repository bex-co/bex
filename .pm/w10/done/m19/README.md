# w10 · m19 — Enforce the "Full internet" agent network tier through a gateway egress proxy

**Worker:** worker10 **Goal:** A workspace that chooses Settings → Agents → **Full internet** gets sandboxes that can reach any public host over HTTP(S), while model-provider hosts stay unreachable except through the credential proxy. Enforcement uses an authenticated HTTP CONNECT proxy in the gateway, not Cilium FQDN rules. **Status:** done (2026-10-28 — shipped `50aec48a1`, `a9b75e553`; verified in production, see done/t005.md)

## Tasks (in order)

| id   | title                                                                                         | est  | depends_on |
| ---- | --------------------------------------------------------------------------------------------- | ---- | ---------- |
| t001 | Gateway egress proxy: Pod-bound HTTP CONNECT (+ plain HTTP) with provider-host/IP denylist — **DONE**     | 2h   | —          |
| t002 | Full-tier session policy: only DNS + gateway hops; VAP + gitops-validate admit the proxy port — **DONE**   | 1h   | t001       |
| t003 | Driver/agent env: HTTPS_PROXY/HTTP_PROXY/NO_PROXY for full-tier sessions; overlay wording — **DONE**       | 45m  | t002       |
| t004 | Enable "Full internet" in Settings → Agents; ADR047 D5 amendment — **DONE**                              | 30m  | t003       |
| t005 | Verify in production: full tier reaches example.com; api.anthropic.com/openrouter.ai refused — **DONE**   | 30m  | t004       |
| t006 | Render parity (environment tier semantics across REST/GraphQL/MCP/dashboard) — **DONE**                  | 20m  | t005       |
| t007 | Simplify — **DONE**                                                                                      | 20m  | t006       |
| t008 | Test coverage — **DONE**                                                                                 | 30m  | t006       |
| t009 | Closeout — **DONE**                                                                                      | 10m  | t007, t008 |

## Definition of done

- In a full-tier workspace, an agent sandbox's `curl https://example.com` succeeds through the proxy. Direct connections (bypassing the proxy) are still dropped by the session policy.
- CONNECT or HTTP to model-provider hosts (api.anthropic.com, api.openai.com, generativelanguage.googleapis.com, openrouter.ai), or to their resolved IPs, through the egress proxy is refused (403), so the platform key can't be bypassed.
- Private, cluster and metadata ranges are refused by the proxy (SSRF guard).
- The proxy authenticates the source Pod as the session's sandbox (the same resolver as the Git and model proxies) and enforces per-session budgets. Non-full-tier sessions cannot use it.
- `registries` and `custom` behaviour is unchanged.
- The Settings UI enables Full internet with honest copy.
- Backend real-DB suite, `make lint`, `scripts/gitops-validate.sh` (apart from the known local-binary false positive), dashboard suite and Prettier are green.

## Source + Goal linkage

- **Source:** promoted from `w10/006` (2026-10-28), filed by w10/m17. The tier was stored but enforced as `registries`, because `bex-api-session-egress` admits only exact FQDN rules and Cilium deny rules can't carve provider FQDNs out of a world rule.
- **Design:** a gateway CONNECT proxy, the same model Codex cloud and Claude Code on the web use for configurable internet access. Tools that ignore proxy env or use raw sockets/SSH can't reach the internet; this is documented.
- **Goal linkage:** ADR047 D5 agent egress; ADR008 AI-native pillar.
- **Expected outcome:** workspaces that need arbitrary hosts (docs sites, private package mirrors, APIs) can opt in, without exposing the platform model key.
- **Why now:** Settings → Agents shows the option as "coming soon"; this finishes the m17 tiering the user asked for.
- **Render parity included (t006):** tier semantics on REST/GraphQL/MCP plus the UI. There is no render.com equivalent.
