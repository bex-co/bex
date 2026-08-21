# w9 · m90 — Security-audit run-1 remediation

**Worker:** worker9 **Goal:** Close the one confirmed vulnerability and the accepted hardening notes from security-audit run-1, so the billing-enforcement boundary holds on every provisioning surface and the defense-in-depth gaps are removed. **Status:** done

## Tasks (in order)

| id   | title                                                                     | est | depends_on                     | status         |
| ---- | ------------------------------------------------------------------------- | --- | ------------------------------ | -------------- |
| t001 | Fix Blueprint apply billing-enforcement bypass (confirmed MEDIUM)         | 45m | —                              | — **DONE**     |
| t002 | Uniform quote-safe PromQL/LogQL escaping                                  | 30m | —                              | — **DONE**     |
| t003 | Generic 500 body in core.WriteErr (stop leaking internal error strings)   | 30m | —                              | — **DONE**     |
| t004 | Single-use nonce on the :8091 mint HMAC (agentsession.Verify)             | 45m | —                              | — **DONE**     |
| t005 | Low-risk hardening sweep: registrycreds scope · nonce order · proxy-CIDR  | 40m | —                              | — **DONE**     |
| t006 | Render parity: billing-refusal + 500 error dialect across REST/GraphQL/MCP | 30m | [t001, t002, t003, t004, t005] | — **DONE**     |
| t007 | Simplify the changed code                                                 | 20m | [t006]                         | — **DONE**     |
| t008 | Test coverage for the shipped fixes                                       | 40m | [t006]                         | — **DONE**     |
| t009 | Closeout                                                                  | 10m | [t008]                         | — **DONE**     |

## Definition of done

- A workspace in the Stripe dunning `enforced`/`recovering` lifecycle state is **refused** on the Blueprint deploy/sync path (create + update) with the same coded billing-enforcement error the interactive create paths return, proven by a regression test; no new paid Service/Postgres/KeyValue is created for such a workspace.
- PromQL/LogQL query builders escape the `"` string-literal terminator uniformly (no builder can be broken out of a quoted literal).
- `core.WriteErr` returns a generic body for unclassified 5xx errors; raw pgx/Kubernetes error strings no longer reach clients (verified by test).
- The gateway→bex-api `:8091` credential-mint HMAC hop rejects a replayed `(timestamp, signature, body)` via a single-use nonce.
- `registrycreds.ResolveCredentialNames` is workspace-scoped; the sshgateway nonce is claimed durably before the in-memory mark; the `BEX_PROXY_PROTOCOL_TRUSTED_CIDRS` narrow-CIDR requirement is documented where operators set it.
- All three backend suites + lint green; error-shape parity re-verified across REST/GraphQL/MCP.

## Source + Goal linkage

- **Source:** Security audit run-1 (`~/security-audit-skill/bex9/run-1/REPORT.md`, `FINDINGS-DETAIL.md`, `findings.json`), handed to w9 by user request 2026-08-20. One confirmed MEDIUM finding + six hardening notes.
- **Goal linkage:** Platform tenant-isolation / billing-integrity posture (the ADR028→ADR077 security-review lineage; ADR040/ADR046 billing enforcement). Keeps bex's fail-closed, single-shared-authz-path design honest across every surface.
- **Expected outcome:** The dunning-enforcement boundary can no longer be bypassed via Blueprint apply (its unbounded-cost impact removed), and the accepted defense-in-depth gaps (query-escaping, error-leakage, mint replay, credential-name scoping, nonce ordering) are closed rather than carried forward to a future review round.
- **Why now:** The Blueprint bypass is a live defeat of an explicit billing-enforcement control with unbounded financial impact wherever production Stripe billing is enabled; batching the cheap hardening notes with it while the audit context is warm avoids a re-discovery round. This is a fix touching the REST/GraphQL/MCP billing-error and 5xx-error surfaces, so **Render parity is included** (t006) — the billing-refusal and generic-500 shapes must match across all three adapters and Render's error dialect.
