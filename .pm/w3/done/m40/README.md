# w3 · m40 — Session egress: phase-split default-deny + per-session allowlist (ADR047 D5)

**Worker:** worker3 **Goal:** agent-session sandboxes keep the `<ws>-sandbox` default-deny posture with a Codex-style phase split — setup may reach package registries, the agent phase reaches only GitHub + the model endpoint (+ the gateway credential hop) — and tenants can widen per session with an explicit allowlist. **Status:** done — live verification PASSED 2026-08-03 on the prod gVisor/Cilium substrate: 54-check matrix (`scripts/verify-sandbox-isolation-live.sh` with BEX_VERIFY_AGENT_DRIVER=1) green — setup/agent egress phase split, gateway-only driver listener identity scoping, per-session allowlist, and all pre-existing isolation checks. Fixture fixes en route (billing-exclude disposable workspaces for the ADR046 gate; workspace A on the pro tier because the hobby sandbox quota admits only two concurrent sandboxes and the agent leg needs a third; policy-stamp readiness wait). Live finding recorded as `w3/011.md`: quota-denied sandbox creation surfaces as an opaque 502 after bex-api's 30s client timeout instead of a quota error.

## Tasks (in order)

| id   | title                                                                          | est | depends_on |
| ---- | ------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Setup-phase vs agent-phase policy split (registries open during setup only) — **DONE**     | 60m | —          |
| t002 | Baseline agent-phase allowlist: GitHub + model endpoint + gateway credential hop — **DONE** | 45m | t001       |
| t003 | Per-session tenant allowlist widening (API field → policy) — **DONE**                      | 60m | t002       |
| t004 | Live verification on the gVisor substrate (extend verify-sandbox-isolation-live) | 45m | t003       | — **DONE**
| t005 | Simplify pass over the egress-policy code — **DONE**                                       | 20m | t004       | — **DONE**
| t006 | Test coverage: phase transitions, allowlist rendering, deny defaults — **DONE**            | 45m | t004       | — **DONE**
| t007 | Closeout                                                                        | 10m | t006       |

## Definition of done

- A session sandbox in the setup phase can reach package registries (and GitHub for clone); on transition to the agent phase the policy narrows to GitHub + the configured model API endpoint + the gateway credential endpoint — enforced by the m35 Cilium DNS/FQDN/SNI machinery, fail-closed.
- Default-deny remains the base (no `allow-all`; the m35 truthfulness rule holds); a session with no extra allowlist gets exactly the baseline.
- The session create surface (w3/m39) accepts an explicit per-session egress allowlist (Codex pattern) that renders into per-sandbox policy; unknown/invalid entries are a named 400, never silently ignored.
- `scripts/verify-sandbox-isolation-live.sh` (or a session-specific sibling) proves on the live substrate: setup-phase registry reachability, agent-phase denial of a non-allowlisted destination, allowlisted-destination reachability, and cross-sandbox isolation unchanged.
- ADR047 D5 note recorded: when the wave-2 metering LLM proxy exists, the model-endpoint allowlist narrows to the proxy.

## Implementation status (2026-08-02)

- **Mechanism (t001–t003):** `lego/backend/internal/sessionegress` renders one namespaced per-session `CiliumNetworkPolicy` keyed by the immutable `bex.co/agent-session` identity, with a one-way setup→agent transition (setup admits the curated/`BEX_AGENT_SETUP_REGISTRIES` package catalog; agent narrows to GitHub + the per-session model endpoint + the in-cluster gateway credential hop). Tenant widening is exact-public-DNS only, `AGENT_SESSION_EGRESS_ALLOWLIST_INVALID` 400 on invalid/duplicate/private/wildcard/URL/excess entries. Wired through `sandbox.AgentSessionLifecycle` and the `agentsessions` create surface (REST/GraphQL/MCP `egressAllowlist`). Platform-side: `deploy/gitops/base/tenant-node-egress.yaml` keeps the structural clusterwide `sandbox-egress-default-deny` and moves positive rules to `sandbox-egress-legacy-allowlist` (which excludes agent-session Pods); `bex-api-session-egress` admission confines bex-api's dynamic Cilium authority.
- **t002 forward note:** ADR047 D5/D6 proxy-narrowing note added to `sessionegress/policy.go` (`ModelEndpointHost`) — when the wave-2 metering LLM proxy ships, the provider resolver returns only that proxy endpoint.
- **t006 tests:** `sessionegress/policy_test.go` (phase split, one-way transition + allowlist/endpoint immutability, provider→FQDN via `agentsessions/egress_test.go`, invalid-entry 400s, and a new golden-shape test asserting every rendered profile is additive-only and never carries a CIDR/entity/wildcard/broad-selector escape that could lift the deny floor). `go test ./...` green, `make lint-backend` 0 issues.
- **t005 simplify:** applied — reuse `gqlutil.StringList` + `store.SandboxNamespace`, shared `privateOrClusterHost` reject list across both validators, dropped per-render re-validation of the static registry catalog, and a shared `matchNames` renderer. Declined (recorded follow-up): collapsing the CNP-annotation vs sandbox-metadata dual state by having `TransitionToAgent` load endpoint+allowlist from the persisted setup policy — a redesign of the security-critical immutability seam, out of scope for a behavior-preserving pass.
- **t004/t007 outstanding:** the live-substrate checks already exist in `scripts/verify-sandbox-isolation.sh` (setup-registry reachability, agent-phase denial, baseline + tenant-widened reachability, cross-sandbox isolation), driven by the `verify-sandbox-isolation-live.sh` disposable-fixture wrapper. The RUN + evidence capture is **operator-gated** and requires the production gVisor+Cilium substrate (the local `kind-bex-mgmt` context has no `gvisor` RuntimeClass or Cilium CRDs). Per t004, this "stays operator-run like m35." Closeout (t007) holds until that run passes and evidence is recorded.

## Source + Goal linkage

- **Source:** [docs/ADR047-cloud-coding-agent-sessions.md](../../../docs/ADR047-cloud-coding-agent-sessions.md) D5; `/pm-brainstorm` decomposition 2026-08-01. Mechanism precedent: w3/m35 (fail-closed Cilium DNS/FQDN/SNI egress, per-sandbox default-deny).
- **Goal linkage:** pillar 5 + tenant isolation — egress control is the primary prompt-injection/exfiltration mitigation in every verified exemplar (Codex, Copilot agent firewall), not in-sandbox policy.
- **Expected outcome:** a hijacked agent cannot exfiltrate beyond the allowlisted destinations; residual (attacker-account-on-approved-service) stays recorded in `.pm/w3/008.md`.
- **Why now:** ADR047 wave 1, parallel with m37–m39; w3/m41's live E2E should run under realistic egress. Render parity omitted: network-policy infrastructure — the API-field surface consistency is covered by m39's parity task; no independent REST/GraphQL/MCP/UI semantics change here beyond the field m39 already checks.
