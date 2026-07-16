# w10 · m4 — Verification & evidence chores: cross-stream note burn-down round 1

**Worker:** worker10 **Goal:** Four aging cross-stream verification/evidence notes land as one chores round (the w7/m37 / w8/m16 pattern, applied across workstreams — w10's spare-capacity charter): the `mfaEnabled` webauthn-stub false positive is fixed, the ADR018 stale-marker sweep runs, the w1/m36 prod ServiceAccount patch is verified/declared, and the orphaned confirmatory Render audit-log capture runs. Absorbs `w4/020`, `w5/013`, `w1/024`, `w4/024`. **Status:** todo

## Tasks (in order)

| id   | title                                                                                       | est | depends_on |
| ---- | -------------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Fix the `mfaEnabled` webauthn-stub false positive (from `w4/020`)                            | 45m | —          |
| t002 | ADR018 stale-marker sweep: cron build-command + email-recovery rows (from `w5/013`)          | 30m | —          |
| t003 | Verify/declare the w1/m36 prod default-SA `imagePullSecrets` patch (from `w1/024`)           | 30m | —          |
| t004 | Confirmatory live Render audit-log capture (from `w4/024`)                                   | 30m | —          |
| t005 | Render parity                                                                                 | 20m | t001       |
| t006 | Simplify                                                                                      | 15m | t005       |
| t007 | Test coverage                                                                                 | 25m | t005       |
| t008 | Closeout                                                                                      | 15m | t007       |

## Definition of done

A password-only user (webauthn credential stub, no enrolled key) reports `mfaEnabled: false` on the owners/members surfaces and the dashboard, with per-shape unit tests; ADR018's cron and email-recovery markers reflect verified current truth; the prod `default` ServiceAccount's `imagePullSecrets` state is either reverted (moot under w7/m36 per-App creds) or declared in `deploy/gitops` — no undeclared drift remains; Render's live audit-log response is captured and diffed against the w4/m26 shape (drift filed as follow-up, or confirmation recorded in `docs/render-artifacts/`); all four source notes closed to their workstreams' `done/`.

## Source + Goal linkage

- **Source:** `/pm-brainstorm` round 19, 2026-07-15 — user directive to give w10 a milestone; bundles the four open cross-stream sub-hour notes (none milestone-worthy alone, all > 1h together): `w4/020` (mfaEnabled derivation, `lego/backend/internal/workspaces/kratos.go` — verified live against dev-4 by w4/m25), `w5/013` (ADR018 cron/email-recovery stale markers), `w1/024` (w1/m36's undeclared prod `kubectl patch`), `w4/024` (w4/m26's skipped live capture, orphaned when w5/m32 closed). Notes moved to their workstreams' `done/` at materialization; this README is their ownership record.
- **Goal linkage:** parity-ledger truthfulness (ADR018 is the loop's instrument — stale markers send future rounds chasing resolved gaps), gitops declared-state integrity (ADR019/ADR022), and a real correctness fix on the members surface (`mfaEnabled` misreports security posture).
- **Expected outcome:** four notes cleared in one round; the members surface stops overstating MFA enrollment; no undeclared prod drift from m36 remains.
- **Why now:** w10 has zero open milestones (m3 closed 2026-07-15) and these notes are aging across four workstreams with no owner momentum; two (t003, t004) need live access that concurrent sessions just proved workable.
- **Render parity:** included — t001 changes a field's semantics on the owners/members REST/GraphQL surfaces and the dashboard; verify all report the corrected value consistently and that Render's own `mfaEnabled` semantics (enrolled second factor, not credential presence) are the anchor.
- **Coordinate with — never duplicate:** `w4/m20/t003` owns the MFA-row-cites-open-m11 + email-recovery ADR018 pointers — t002 takes the two rows `w5/013` scoped and leaves m20's rows alone.
