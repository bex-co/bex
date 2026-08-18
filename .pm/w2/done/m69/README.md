# w2 · m69 — ADR062 phase 1: credential-injecting model proxy (BYO key leaves the sandbox)

**Worker:** worker2 **Goal:** the tenant's BYO model-provider key never enters the agent sandbox — a gateway-side proxy injects it on the upstream hop only, the sandbox env carries a useless placeholder, and the tenant egress policy loses its one high-value destination. **Status:** done

## Tasks (in order)

| id   | title                                                                       | est | depends_on |
| ---- | --------------------------------------------------------------------------- | --- | ---------- |
| t001 | bex-api: gateway-only model-credential resolve endpoint (D3) — **DONE**      | 45m | —          |
| t002 | gateway: model-proxy listener — source-pod auth + header injection — **DONE** | 90m | t001       |
| t003 | dispatch + egress: placeholder env, vendor host out, proxy carve-out — **DONE** | 60m | t002       |
| t004 | driver: per-agent base-URL routing onto the proxy (D5, D7) — **DONE**        | 45m | t003       |
| t005 | Simplify — `/simplify` over the changed code — **DONE**                      | 30m | t004       |
| t006 | Test coverage — proxy auth/injection/narrowing + failure modes — **DONE**    | 45m | t005       |
| t007 | Closeout — **DONE**                                                          | 15m | t006       |

## Definition of done

With the proxy enabled on a live session: (1) the sandbox pod spec and every process env inside it contain **no** real provider credential — `BEX_AGENT_MODEL_API_KEY` (or the agent-native var) holds a placeholder that fails direct use against the vendor API; (2) an agent turn completes end to end with model traffic observed only on the sandbox→gateway hop, and the gateway's upstream request carries the injected credential to the session's registered `ModelEndpointHost` and nothing else; (3) the rendered per-session Cilium policy no longer admits the vendor model host from the tenant pod and admits the proxy service instead; (4) a request to the proxy from a pod that is not the session's bound sandbox is refused; (5) with the feature env unset, behavior is byte-identical to today (real key in pod env, vendor host in policy). All backend suites green.

**Status: DONE (2026-08-15).** Shipped env-gated OFF (`BEX_AGENT_MODEL_PROXY_URL` unset ⇒ byte-identical to pre-ADR062). Every DoD clause is pinned by automated tests: (1) `TestCreateProxyModeKeepsRealKeyOutOfTheSandbox` (+ driver routing tests); (2) `TestProxyInjectsRealKeyAndPinsHost` + the OAuth/OpenAI/Gemini scheme cases (`internal/sshgateway/modelproxy`, an end-to-end walk gateway→bex-api mint→injected upstream with a stub vendor); (3) `TestModelProxyNarrowingDropsVendorHostAndAddsGatewayPort` (`internal/sessionegress`); (4) `TestProxyRefusesForeignOrMislabeledPod` + `TestAuthorizeSessionPodBindsExactSession` + `TestProxyRefusesNonLiveSession`; (5) `TestCreateWithoutProxyIsByteIdentical` + the unchanged `sessionegress` baseline tests. Backend `go test ./...` + the 30-test driver suite green. **Live enablement** (a real agent turn reaching a real vendor through the proxy, mirroring m68's deferred live walk) is the operational step once `BEX_AGENT_MODEL_PROXY_URL` is provisioned in prod — infeasible pre-enablement here, exactly as m68's live hibernate→rehydrate walk was. Phase 2 (`agent_token_units` metering) rides the same choke point and is a later milestone.

## Source + Goal linkage

- **Source:** [docs/ADR062-sandbox-credential-vault.md](../../../docs/ADR062-sandbox-credential-vault.md) (Proposed 2026-08-15) — phase 1 (D1–D5, D7). Closes the residual risk recorded in `w3/done/m35/done/t003.md` ("Credential Vault/MITM proxying … documented as residual risk"); implements the forward note in `lego/backend/internal/sessionegress/policy.go` `ModelEndpointHost`. OpenSandbox's own Credential Vault was evaluated and rejected for the gVisor substrate (its egress sidecar needs the iptables `nat` table runsc lacks — the same fact that shaped m35).
- **Goal linkage:** pillar 5 (cloud coding-agent sessions, ADR047) — security hardening of the one credential ADR047 D7 knowingly left inside the sandbox; continues the ADR028→ADR072 security lineage's credential-confinement theme (Git token D2, exec/attach tickets).
- **Expected outcome:** supply-chain code running inside an agent sandbox (install hooks, tool processes) can no longer read a reusable provider credential from `/proc` or inherited env; the encoded-exfiltration gap in the literal-match scrubs stops mattering for this secret; ADR047 D6 phase 2 (token metering, `agent_token_units`) gets its choke point for free.
- **Why now:** ADR062 was just accepted as the decision record after the user challenged the BYO-key exposure (2026-08-15); the ADR059 hibernation work (m67/m68) settled the sandbox lifecycle this proxy must compose with, so the seam is stable; phase 2 billing is blocked behind this choke point existing.
- **Render parity omitted:** pure mechanism/infra — no REST/GraphQL/MCP field, error shape, or dashboard surface changes; the agent-sessions API contract is untouched (Render has no equivalent surface either).
