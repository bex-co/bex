# w2 · m536 — Focused security review, CLI/SDK compatibility suite & production verification of sandbox credential brokering

**Worker:** worker2 **Goal:** Qualify sandbox group credential brokering (`w2/m532`–`m535`) with a focused adversarial security review ADR, pinned Render CLI and Python SDK contract suites, an isolation-matrix leg, and a live production walk on `api.bex.co` so the parity ledgers (`ADR018`, `ADR104`, `cli-compatibility-checklist.md`) turn `✅` with dated evidence. **Status:** done

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Run focused adversarial security review of the credential control plane, OpenBao custody, and TLS-intercepting broker (`docs/ADR123-security-review-sandbox-credentials.md`) and fix any findings — **DONE** | 1h30m | `w2/m533`, `w2/m535` |
| t002 | Add pinned Render CLI contract tests (`lego/cli/testdata/sandbox-credentials-contract.json` + `sandbox_credentials_contract_test.go`) — **DONE** | 45m | t001 |
| t003 | Extend `scripts/render-sdk-compat.sh` with Render Python SDK credential round-trips and add a broker-isolation leg to `isolation-matrix.yml` — **DONE** | 1h | t001 |
| t004 | Run full production walk on `api.bex.co` with unmodified `bex`/`render` CLI, Render SDK, and real sandbox Pods, then dispose all fixtures — **DONE** | 1h | t002, t003 |
| t005 | Update `ADR018`, `ADR104`, `cli-compatibility-checklist.md`, and `docs/AGENTS.md` to `✅` with dated production evidence and recorded `HTTPS_PROXY` caveat — **DONE** | 30m | t004 |
| t006 | Simplify — **DONE** | 15m | t005 |
| t007 | Test coverage — **DONE** | 30m | t005 |
| t008 | Closeout — **DONE** | 10m | t006, t007 |

## Definition of done

- Focused security review ADR is committed with every attack vector tested and passing (zero unmitigated High/Medium findings; any accepted residual documented).
- Pinned CLI contract tests (`lego/cli`) and extended `scripts/render-sdk-compat.sh` pass, and the isolation matrix proves a sandbox in group A cannot trigger injection of group B's credentials or bypass `networkPolicy`.
- Live production walk proves `header`, `basic`, `cookie`, and `query` injection, method/path filtering, live version restore and disable cutoff, and clean fixture teardown with zero secret leakage into sandbox `env`, rootfs, snapshots, API responses, or logs.
- `ADR018`, `ADR104`, and `cli-compatibility-checklist.md` reflect `✅` with dated production evidence and the explicitly noted `HTTPS_PROXY` vs transparent-443 difference awaiting user sign-off.

## Source + Goal linkage

- **Source:** `/pm-brainstorm one aspect of render parity for w2` (2026-11-18), materialized via `/pmx them all to w2`. Follows [ADR104 D1 & D9](../../../../docs/ADR104-sandbox-render-parity.md#d1--the-parity-target-is-renders-documented-contract-graded-row-by-row) (`✅` only when verified live; SDK is part of the contract) and bex's focused security review lineage ([ADR117](../../../../docs/ADR117-security-review-api-key-roles.md), [ADR118](../../../../docs/ADR118-security-review-scoped-api-keys.md)).
- **Goal linkage:** Satisfies bex's security bar for TLS-intercepting secret injection and closes the final non-excluded omission row in Render's Public API with live CLI + SDK proof.
- **Expected outcome:** Sandbox Group Credential Brokering is production-verified, security-audited, regression-pinned in CI, and marked `✅` across all parity ledgers.
- **Why now:** An outbound TLS-intercepting secret broker must not be declared done on unit tests alone; closing out `w2/m532`–`m535` with an adversarial review and live CLI/SDK proof prevents silent regressions or security gaps.
- **Render parity:** Omitted — this milestone *is* the Render CLI/SDK parity verification and focused security audit (matching `w2/m79` and `w2/m251` precedent).
- **Depends on:** `w2/m533`, `w2/m535`.
- **Brainstorm recommendations, not user decisions:** Records the `HTTPS_PROXY` + Broker CA vs transparent-443 difference and deterministic lexicographical `credentialId` overlap tie-breaking as open items awaiting user sign-off in `ADR018`/`ADR104`.

## Unblock & production verification record (2026-11-22)

- **Pin gate cleared:** production pin `de0ed1cbfe7f` (`914f6e5540`, `ghcr.io/bex-co/bex-operator@sha256:7bbf4b6731104a90b4fde13f6e4588bc717749384fee6b0471f018b784ab7c59`) carries `48b78ea2c5` (`git merge-base --is-ancestor 48b78ea2c5 de0ed1cbfe7f` exited 0); `bex-sandbox-broker-ca` Ready, `bex-ssh-gateway` listening on `:8086`, Cilium broker policies `True`, `bex-sandbox-pods` VAP active.
- **Live walk (`t004`):** in `bex-canary` (`tea-daif693dqjvc73e7as3g`), `scripts/render-sdk-compat.sh` passed 41/41 Python (`render-sdk==1.8.0`, including all 7 `*_sandbox_group_credential*` operations) and 35/35 TypeScript (`@renderinc/sdk@1.8.0`); pinned `bex` CLI v2.36 passed all 7 `sandbox-groups credentials` commands with zero secret leakage; real sandbox Pods verified `header` (`curl`, `python3`, `node`), `basic`, `cookie`, and `query` injection, method/path filtering, live `update`/`enabled=false`/`restore` cutoff within 6 s, selective MITM pass-through on uncredentialed hosts, sibling-group/`deny-all`/`allow-list` isolation, and zero-secret snapshot capture + post-restore injection; all fixtures disposed with clean empty readback.
- **Ledgers & closing tasks (`t005`–`t008`):** `ADR018`, `ADR104`, `cli-compatibility-checklist.md`, `ADR122`, and `docs/AGENTS.md` updated to `✅`/`[x]` with dated evidence and explicit open sign-off caveats (`HTTPS_PROXY` + Broker CA vs transparent-443, lexicographical `credentialId` tie-breaking); Simplify audit clean; CI-matching test suites, `gitops-validate.sh`, `github-actions-validate.sh`, `ci_test_inputs.py`, `shellcheck`, `pm-board-validate.py`, and `docs-links-validate.sh` all green.
