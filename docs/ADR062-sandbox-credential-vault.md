# ADR062 — Sandbox credential vault: keep the BYO model key out of the untrusted agent process tree

**Status:** Accepted (2026-08-15), hardened by [ADR064](ADR064-security-review-round10.md) (2026-08-16). Phase 1 (D1–D5, D7) shipped in **w2/m69** and is now mandatory for agent-session mutation: unset `BEX_AGENT_MODEL_PROXY_URL` disables create/steer/rehydrate; the direct-key fallback was removed and the sandbox lifecycle admits only the exact session placeholder. ADR064 additionally binds each agent profile to a platform-registered provider origin, authorizes only inference operations, bounds bodies/streams/concurrency, and revalidates lifecycle on every provider exchange. Phase 2 (token metering, ADR047 D6 `agent_token_units`) rides the same choke point and is a later milestone. This ADR decides how the tenant's reusable model-provider credential stops being readable by repository code and tool processes inside the agent sandbox.

---

## Context

### The historical exposure, from the pre-ADR062 code

Before phase 1 shipped, the BYO model key's custody chain was strong until the last hop, where it entered the untrusted process tree in the clear:

1. **At rest** — OpenBao, workspace-scoped path `agent-sessions/<ws>/model-key` (`lego/backend/internal/agentsessions/service.go` `modelKeySecretPath`). Fetch is fail-closed (`ErrSecretsUnavailable` aborts the create); the key never lands in the DB, audit events, OpenSandbox metadata/labels, or the create response.
2. **Delivery** — bex-api injects the plaintext as pod-spec env `BEX_AGENT_MODEL_API_KEY` in the sandbox create request (`lego/backend/internal/sandbox/service.go` `CreateAgentSessionSandbox`).
3. **In the sandbox** — the driver reads it at startup and deletes it from its own env (`lego/agent-image/driver/src/credentials.ts`), but must then re-inject it into the agent child's environment under the agent-native name (`ANTHROPIC_API_KEY` / `CLAUDE_CODE_OAUTH_TOKEN`, routed by credential shape in `config.ts`). The agent spawns tools and repository code (install hooks, build scripts) that inherit that environment — and the pod's entrypoint keeps the original value visible in `/proc/1/environ` to every same-UID process regardless of the driver's delete.

Compensating controls exist and stay: default-deny Cilium egress with a per-session allowlist (`internal/sessionegress`), scrub-before-deliver + fail-closed pre-push history scan (ADR056 F6), pre-snapshot scrub (ADR047 D2). But the scrubs are literal-match (raw + JSON-escaped forms) — an encoded copy (base64 in a commit, in the conversation stream) passes them. The root fact stands: **a reusable, workspace-lifetime provider credential is readable by arbitrary tenant code**, and the model endpoint it works against is precisely the host the egress policy allowlists.

ADR047 solved the identical problem for the **GitHub token** with the right shape: the token never enters the sandbox — the gateway's Git smart-HTTP proxy injects it on the upstream hop only (`internal/sshgateway/agentcred`). The model key is the one credential that did not get this treatment in v1, and ADR047 knowingly deferred the fix to "the metering proxy."

### OpenSandbox Credential Vault (surveyed 2026-08-15)

OpenSandbox ships a purpose-built answer ([open-sandbox.ai/guides/credential-vault](https://open-sandbox.ai/guides/credential-vault)): the sandbox env carries **fake** credential values; real secrets are written by the host-side SDK into the egress sidecar's process-local memory; the sidecar transparently MITMs outbound HTTPS, matches requests against bindings (scheme/host/method/path → bearer / basic / apiKey header / custom headers / body-substitution), and injects the real credential on the wire. The sandbox process tree never sees the secret. Requirements: opensandbox-server ≥ 0.2.0 (we run 0.2.2), **egress sidecar ≥ 1.1.1 in `dns+nft` mode**, network policy `defaultAction=deny`, no coexisting service-mesh sidecar. Vault entries do not survive pod deletion (pause/resume restarts the sidecar empty; a trusted client must re-provision).

It is the textbook mechanism for this threat — and it is **unusable on bex's substrate**:

- **The gVisor blocker.** The egress sidecar's `dns+nft` interception needs the iptables `nat` table, which gVisor (runsc) does not implement. This is not a new discovery: it is the recorded reason w3/m35/t003 built sandbox egress on **Cilium DNS-L7 + `toFQDNs` outside the guest**, with "no OpenSandbox egress sidecar dependency" as an explicit acceptance criterion. bex's sandbox pool is gVisor (ADR042) and stays gVisor.
- **Parallel enforcement stack.** Adopting the vault means running OpenSandbox's egress sidecar alongside (or instead of) the Cilium policies `internal/sessionegress` renders — two allowlists to keep coherent, or a migration off the stack m35 hardened and verified on gVisor.
- **Integration surface we don't have.** Only SDK access is documented (no REST contract for the vault write path); our Go client (`internal/sandbox/client.go`) is a deliberate minimal REST subset, and our controller is a pinned bex fork (`v0.2.0-bex-snapjobns`). MITM CA distribution into the sandbox image is undocumented.
- **Lifecycle friction.** In-memory-only entries must be re-provisioned on every pod recreation — every ADR059 rehydration and every OpenSandbox-side restart adds a host-side re-write step that fails open to "agent silently unauthenticated."

### What the repo already points at

`internal/sessionegress/policy.go` (`ModelEndpointHost`) carries a forward note written before this ADR: when the ADR047 D6 phase-2 metering LLM proxy ships, the per-session provider resolver returns **only the in-cluster proxy endpoint**, direct vendor hosts disappear from rendered policies, and "one egress choke point then owns both token metering and exfiltration containment." The proxy is not just an alternative to the vault — it is the already-reserved seam, and the credential-injection half can ship **before** the metering half.

## Options

|  | A — OpenSandbox Credential Vault | B — bex-owned credential-injecting model proxy | C — status quo + provider-side restricted keys |
| --- | --- | --- | --- |
| Secret in sandbox | never (fake env value) | never (placeholder env value) | yes (plaintext env) |
| Works on gVisor | **no** (nft/nat, m35/t003) | yes (no in-guest interception) | yes |
| Egress stack | new sidecar (`dns+nft`), overlaps Cilium | narrows the existing Cilium policy | unchanged |
| Survives ADR059 rehydration | re-provision required each pod | stateless proxy, nothing to re-provision | n/a |
| Token metering (ADR047 D6 ph. 2) | not provided | same choke point, phase 2 | not provided |
| Build cost | SDK/fork integration + CA distribution + substrate change | one new gateway listener reusing proven patterns | none |
| MITM CA in sandbox | required, distribution undocumented | none (client explicitly targets the proxy base URL) | none |

Option C alone is not a fix — it shrinks the blast radius (a scoped project key with a spend cap instead of an account key) but leaves a reusable credential readable by tenant code. It stays as **guidance**, complementary to B.

## Decision

**Option B**: route agent model traffic through a bex-owned credential-injecting proxy on the isolated SSH gateway, so the real key never enters the sandbox. The vault (A) is rejected for bex's substrate, with explicit re-evaluate triggers.

### D1 — Placement: a new listener on the isolated gateway, mirroring the Git credential proxy

The proxy is a cluster-internal plain-HTTP listener on `bex-ssh-gateway` (pattern and trust model of `BEX_AGENT_CREDENTIAL_ADDR`'s Git smart-HTTP proxy, ADR047 D2): no edge route, reachable only from sandbox namespaces via the session egress policy. The gateway is already the platform's credential-custody plane for agent sessions (Git tokens, exec tickets, attach tickets); bex-api keeps authorizing and minting, and still never joins the sandbox's network path.

### D2 — Sandbox-side identity: source-pod authentication + session binding; no real secret in the sandbox

The sandbox authenticates to the proxy the way it does to the Git proxy: the gateway resolves the **direct source Pod** and matches it against the session's recorded sandbox binding (namespace + exact pod + session id). The env var the agent reads (`ANTHROPIC_API_KEY` etc.) carries a per-session **placeholder** — useless off-pod by construction, since the proxy trusts pod identity, not the placeholder string. Stealing the placeholder yields nothing; there is no reusable credential to exfiltrate.

### D3 — Key custody: unchanged OpenBao path, gateway fetches through bex-api on demand

The key stays at `agent-sessions/<ws>/model-key` in OpenBao. The gateway does not get OpenBao access: it requests the credential for a verified session from bex-api's gateway-only internal mint endpoint (the `BEX_AGENT_CREDENTIAL_API_URL` pattern) on every provider exchange, so a terminal/canceled session cannot ride a cached credential. Injection happens on the upstream hop only, onto the platform-registered provider origin for the selected agent profile. The proxy accepts only the profile's inference methods and paths and never follows the sandbox to a caller-selected host.

### D4 — Egress narrowing: vendor hosts leave the tenant policy

With the proxy live, `sessionegress` stops admitting the vendor model host from tenant pods and instead admits the proxy service (a second cluster-internal carve-out beside `credentialGatewayHost`). This **strengthens** D5 of ADR047: the one previously-allowlisted host a stolen key worked against disappears from the sandbox's reachable set. The `ModelEndpointHost` forward note is implemented, not amended.

### D5 — Client routing: per-agent base-URL override, driver-owned

Each supported agent adapter is pointed at the proxy through its reviewed native provider mechanism: `ANTHROPIC_BASE_URL` for claude-code-acp (also covering the `sk-ant-oat` OAuth-token path), ACP `providers/set` for codex-acp, and the Gemini CLI base-URL equivalent. Codex is bound explicitly before `session/new` because `OPENAI_BASE_URL` alone can leave the app server on its built-in public OpenAI provider; the provider-set request supplies the exact session proxy URL and only the session placeholder as authorization. The driver's existing credential plumbing (`BEX_AGENT_MODEL_API_KEY`, `BEX_AGENT_MODEL_API_KEY_ENV`, the scrub manager) is retained; only the value it carries changes from the real key to the placeholder. An agent binary that cannot honor a base-URL override cannot join the proxy path and is not offered until it can (fail closed, per-profile).

### D6 — Phase 2 rides the same choke point: token metering

Phase 1 is a byte-transparent authenticating pass-through (inject `Authorization`/`x-api-key`, stream the response, touch nothing else). Phase 2 parses provider usage frames (SSE `usage` blocks) at the same seam to source `agent_token_units` (ADR047 D6), unlocking bundled-token billing — the reason ADR047 called this "the metering proxy." Phase 1 does not wait for phase 2.

### D7 — Scrub machinery is retained as defense in depth

Scrub-before-deliver, the pre-push history scan, and the pre-snapshot hook all remain: they now protect a placeholder (and any other secret a tenant pastes into a session) rather than being the last line for a live key. ADR059 rehydration needs no credential step at all — the proxy is stateless and pod identity re-binds on redispatch.

### D8 — Option A disposition: rejected for the substrate, with re-evaluate triggers

The OpenSandbox Credential Vault is the right design on a substrate its egress sidecar supports; bex's gVisor pool is not one. Re-evaluate if: (1) gVisor ships working nftables `nat` for the sidecar's interception path, (2) OpenSandbox moves egress enforcement host-side (out of the guest's netfilter), or (3) bex's sandbox pool leaves gVisor (ADR042 revisit). Even then, the metering requirement (D6) still argues for owning the choke point.

## Consequences

- **Positive:** the last reusable credential inside the sandbox becomes a non-credential; the tenant egress allowlist loses its only high-value destination; token metering gets its seam for free; hibernation/rehydration needs no credential re-provisioning; every mechanism reused (source-pod auth, gateway custody, internal mint, Cilium carve-out) already has a shipped, tested precedent in this repo.
- **Negative / accepted:** the model path gains a platform hop — gateway availability now gates agent turns (mitigation: the listener is horizontally scaled); the gateway's blast radius grows by request-lifetime model-key custody; per-agent base-URL and registered-operation support become hard onboarding criteria for new profiles.
- **Residual:** tenant code can use the approved inference APIs while its session is live. Concurrency, byte, duration, and provider-operation limits bound the platform/confused-deputy surface; phase-2 metering and provider-side spend caps bound economic use.

## Non-goals

- Proxying the GitHub token (already confined, ADR047 D2) or platform-internal tickets (already single-use HMAC/Ed25519).
- A general-purpose tenant secret-injection service for arbitrary user workloads — this ADR covers the agent-session model credential only.
- DLP on approved egress content (the m35/t003 residual note's other half): the proxy inspects only what phase 2 metering needs.

## References

- ADR047 D2 (Git credential proxy — the reused pattern), D5 (egress), D6 (metering proxy reservation), D7 (BYO key).
- `.pm/w3/done/m35/done/t003.md` — the gVisor/nat blocker and the recorded residual risk this ADR closes.
- `lego/backend/internal/sessionegress/policy.go` `ModelEndpointHost` — the pre-existing forward note this ADR implements.
- OpenSandbox Credential Vault guide: <https://open-sandbox.ai/guides/credential-vault> (surveyed 2026-08-15; server 0.2.2 deployed, egress sidecar not deployed).
