# w7 · m57 — Security audit round 3: post-2026-07-20 surfaces (billing · static plane · tenant namespaces)

**Worker:** worker7 **Goal:** give the three large surfaces shipped since the last audit window — the Stripe billing lifecycle (money + an unauthenticated public endpoint), the static-site serving/publish plane, and per-tenant namespaces — the same evidence-backed adversarial review the earlier platform got, and produce a fresh follow-up register that becomes w7's next work queue. **Status:** todo

## Tasks (in order)

| id   | title                                                                                                | est | depends_on             |
| ---- | ---------------------------------------------------------------------------------------------------- | --- | ---------------------- |
| t001 | Freeze the audit window + exclusion map                                                              | 45m | —                      |
| t002 | Billing plane audit: webhook intake, enforcement/recovery, key custody, outbox-seal integrity        | 90m | t001                   |
| t003 | Static serving/publish plane audit: S3 identity separation, publish Job, alias-authority residuals   | 60m | t001                   |
| t004 | Tenant-namespace plane audit: NamespaceReconciler authority beyond m35 t013, quota/label/prune paths | 60m | t001                   |
| t005 | Cross-cutting sweep: in-window secret custody/rotation, RBAC/NetworkPolicy/CI-guard drift            | 60m | t001                   |
| t006 | Write the register: severities + evidence, one disposition per finding                               | 60m | t002, t003, t004, t005 |
| t007 | Simplify                                                                                             | 30m | t006                   |
| t008 | Test coverage: regression tests for every fixed-in-place finding                                     | 45m | t006                   |
| t009 | Closeout                                                                                             | 15m | t008                   |

## Definition of done

A fresh ADR028-format register exists in `docs/` (next free ADR number, expected ADR045) covering every in-window surface in the t001 scope map. Every finding has a severity + concrete evidence (file:line, live capture, or reproduction) and exactly one disposition: fixed in place (trivial), filed as an owned follow-up on the `.pm` board, or recorded as accepted risk with rationale. The scope map proves no double-coverage of work owned by `w3/m35`, `w7/m54`, `w7/m55`, `w7/m56`, `w3/007`, or `w3/008`. Zero findings on a plane is a valid outcome **only** with sweep evidence showing that plane was actually probed (what was checked, how, and what held). Every fixed-in-place finding has a regression test asserting the pre-fix failure mode.

## Source + Goal linkage

- **Source:** promotes `w7/010.md` (trigger-gated audit follow-on, filed by `/pm-brainstorm for w7` 2026-07-30). Its written trigger — `w3/m35` closes — fired 2026-07-30; scoped per the note to what m35 did **not** cover. Materialized from `/pm-brainstorm more for w7` 2026-07-30.
- **Goal linkage:** GOAL.md V0 #7 (security review) — w7's founding charter; the third pass in the ADR028 → `w1/m53` audit lineage, keeping the security review continuous rather than one-shot.
- **Expected outcome:** the billing plane (production-live since 2026-07-28, handling money via an unauthenticated public webhook), the static plane, and the tenant-namespace plane get an independent adversarial pass; the register's findings become owned board items instead of latent unknowns.
- **Why now:** the trigger fired today — waiting was only ever about not double-covering `w3/m35`'s owned sandbox hardening, and that reason is gone with m35 deployed and adversarially verified. The ADR028/m53 registers are empty, so this audit is the only mechanism that can surface new unowned security debt.
- **Render parity:** omitted — audit work with no REST/GraphQL/MCP/UI surface change (the m30/m55 precedent). Any surface-touching fix a finding motivates is filed as follow-up work, where parity applies to *that* work.
