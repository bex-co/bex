# w3 · m40 — Session egress: phase-split default-deny + per-session allowlist (ADR047 D5)

**Worker:** worker3 **Goal:** agent-session sandboxes keep the `<ws>-sandbox` default-deny posture with a Codex-style phase split — setup may reach package registries, the agent phase reaches only GitHub + the model endpoint (+ the gateway credential hop) — and tenants can widen per session with an explicit allowlist. **Status:** todo

## Tasks (in order)

| id   | title                                                                          | est | depends_on |
| ---- | ------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Setup-phase vs agent-phase policy split (registries open during setup only)     | 60m | —          |
| t002 | Baseline agent-phase allowlist: GitHub + model endpoint + gateway credential hop | 45m | t001       |
| t003 | Per-session tenant allowlist widening (API field → policy)                      | 60m | t002       |
| t004 | Live verification on the gVisor substrate (extend verify-sandbox-isolation-live) | 45m | t003       |
| t005 | Simplify pass over the egress-policy code                                       | 20m | t004       |
| t006 | Test coverage: phase transitions, allowlist rendering, deny defaults            | 45m | t004       |
| t007 | Closeout                                                                        | 10m | t006       |

## Definition of done

- A session sandbox in the setup phase can reach package registries (and GitHub for clone); on transition to the agent phase the policy narrows to GitHub + the configured model API endpoint + the gateway credential endpoint — enforced by the m35 Cilium DNS/FQDN/SNI machinery, fail-closed.
- Default-deny remains the base (no `allow-all`; the m35 truthfulness rule holds); a session with no extra allowlist gets exactly the baseline.
- The session create surface (w3/m39) accepts an explicit per-session egress allowlist (Codex pattern) that renders into per-sandbox policy; unknown/invalid entries are a named 400, never silently ignored.
- `scripts/verify-sandbox-isolation-live.sh` (or a session-specific sibling) proves on the live substrate: setup-phase registry reachability, agent-phase denial of a non-allowlisted destination, allowlisted-destination reachability, and cross-sandbox isolation unchanged.
- ADR047 D5 note recorded: when the wave-2 metering LLM proxy exists, the model-endpoint allowlist narrows to the proxy.

## Source + Goal linkage

- **Source:** [docs/ADR047-cloud-coding-agent-sessions.md](../../../docs/ADR047-cloud-coding-agent-sessions.md) D5; `/pm-brainstorm` decomposition 2026-08-01. Mechanism precedent: w3/m35 (fail-closed Cilium DNS/FQDN/SNI egress, per-sandbox default-deny).
- **Goal linkage:** pillar 5 + tenant isolation — egress control is the primary prompt-injection/exfiltration mitigation in every verified exemplar (Codex, Copilot agent firewall), not in-sandbox policy.
- **Expected outcome:** a hijacked agent cannot exfiltrate beyond the allowlisted destinations; residual (attacker-account-on-approved-service) stays recorded in `.pm/w3/008.md`.
- **Why now:** ADR047 wave 1, parallel with m37–m39; w3/m41's live E2E should run under realistic egress. Render parity omitted: network-policy infrastructure — the API-field surface consistency is covered by m39's parity task; no independent REST/GraphQL/MCP/UI semantics change here beyond the field m39 already checks.
