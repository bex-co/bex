# w2 · m536 — Focused security review, CLI/SDK compatibility suite & production verification of sandbox credential brokering

**Worker:** worker2 **Goal:** Qualify sandbox group credential brokering (`w2/m532`–`m535`) with a focused adversarial security review ADR, pinned Render CLI and Python SDK contract suites, an isolation-matrix leg, and a live production walk on `api.bex.co` so the parity ledgers (`ADR018`, `ADR104`, `cli-compatibility-checklist.md`) turn `✅` with dated evidence. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Run focused adversarial security review of the credential control plane, OpenBao custody, and TLS-intercepting broker (`docs/ADR<next>-security-review-sandbox-credentials.md`) and fix any findings | 1h30m | `w2/m533`, `w2/m535` |
| t002 | Add pinned Render CLI contract tests (`lego/cli/testdata/sandbox-credentials-contract.json` + `sandbox_credentials_contract_test.go`) | 45m | t001 |
| t003 | Extend `scripts/render-sdk-compat.sh` with Render Python SDK credential round-trips and add a broker-isolation leg to `isolation-matrix.yml` | 1h | t001 |
| t004 | Run full production walk on `api.bex.co` with unmodified `bex`/`render` CLI, Render SDK, and real sandbox Pods, then dispose all fixtures | 1h | t002, t003 |
| t005 | Update `ADR018`, `ADR104`, `cli-compatibility-checklist.md`, and `docs/AGENTS.md` to `✅` with dated production evidence and recorded `HTTPS_PROXY` caveat | 30m | t004 |
| t006 | Simplify | 15m | t005 |
| t007 | Test coverage | 30m | t005 |
| t008 | Closeout | 10m | t006, t007 |

## Definition of done

- Focused security review ADR is committed with every attack vector tested and passing (zero unmitigated High/Medium findings; any accepted residual documented).
- Pinned CLI contract tests (`lego/cli`) and extended `scripts/render-sdk-compat.sh` pass, and the isolation matrix proves a sandbox in group A cannot trigger injection of group B's credentials or bypass `networkPolicy`.
- Live production walk proves `header`, `basic`, `cookie`, and `query` injection, method/path filtering, live version restore and disable cutoff, and clean fixture teardown with zero secret leakage into sandbox `env`, rootfs, snapshots, API responses, or logs.
- `ADR018`, `ADR104`, and `cli-compatibility-checklist.md` reflect `✅` with dated production evidence and the explicitly noted `HTTPS_PROXY` vs transparent-443 difference awaiting user sign-off.

## Source + Goal linkage

- **Source:** `/pm-brainstorm one aspect of render parity for w2` (2026-11-18), materialized via `/pmx them all to w2`. Follows [ADR104 D1 & D9](../../../docs/ADR104-sandbox-render-parity.md#d1--the-parity-target-is-renders-documented-contract-graded-row-by-row) (`✅` only when verified live; SDK is part of the contract) and bex's focused security review lineage ([ADR117](../../../docs/ADR117-security-review-api-key-roles.md), [ADR118](../../../docs/ADR118-security-review-scoped-api-keys.md)).
- **Goal linkage:** Satisfies bex's security bar for TLS-intercepting secret injection and closes the final non-excluded omission row in Render's Public API with live CLI + SDK proof.
- **Expected outcome:** Sandbox Group Credential Brokering is production-verified, security-audited, regression-pinned in CI, and marked `✅` across all parity ledgers.
- **Why now:** An outbound TLS-intercepting secret broker must not be declared done on unit tests alone; closing out `w2/m532`–`m535` with an adversarial review and live CLI/SDK proof prevents silent regressions or security gaps.
- **Render parity:** Omitted — this milestone *is* the Render CLI/SDK parity verification and focused security audit (matching `w2/m79` and `w2/m251` precedent).
- **Depends on:** `w2/m533`, `w2/m535`.
- **Brainstorm recommendations, not user decisions:** Records the `HTTPS_PROXY` + Broker CA vs transparent-443 difference and deterministic lexicographical `credentialId` overlap tie-breaking as open items awaiting user sign-off in `ADR018`/`ADR104`.
