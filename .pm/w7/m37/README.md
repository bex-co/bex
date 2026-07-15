# w7 · m37 — Credential lifecycle hardening chores

**Worker:** worker7 **Goal:** the four documented-but-unowned credential-lifecycle follow-ups from ADR019/ADR013 land: a scoped (non-`system:masters`) operator kubeconfig, alerting before the admin cert expires, and written runbooks for CA rotation and OpenBao root-token/Shamir rekey. **Status:** todo

## Tasks (in order)

| id   | title                                                    | est | depends_on               |
| ---- | -------------------------------------------------------- | --- | ------------------------ |
| t001 | Narrowed non-`system:masters` operator kubeconfig        | 60m | —                        |
| t002 | Admin-cert expiry alert rule (firing-tested)             | 45m | —                        |
| t003 | CA-rotation runbook                                      | 45m | —                        |
| t004 | OpenBao root-token / Shamir-rekey runbook                | 45m | —                        |
| t005 | Simplify                                                 | 30m | t001, t002, t003, t004   |
| t006 | Test coverage                                            | 45m | t001, t002, t003, t004   |
| t007 | Closeout                                                 | 15m | t006                     |

## Definition of done

A scoped operator kubeconfig exists, works for day-to-day ops, and is documented as the default credential (the `system:masters` cert relegated to break-glass); a Prometheus alert fires ahead of admin-cert expiry (proven by a firing test); both runbooks are written, indexed, and reference real commands against the current substrate; ADR019's three `_Follow-up:_` markers and ADR013's "runbook not yet built" line are gone.

## Source + Goal linkage

- **Source:** `/pm-brainstorm` round 12, 2026-07-15 — docs miner (docs/ADR019-infra-credentials.md:80-82; docs/ADR013-secrets.md:111); grouped per the w1/m23 / w1/m30 / w7/m10 chores pattern (each item ~sub-hour-to-an-hour).
- **Goal linkage:** GOAL.md #7 security hardening; credential-custody hygiene (ADR019's trust chain).
- **Expected outcome:** losing/expiring the admin cert stops being a silent time bomb; privileged-credential use is scoped and documented.
- **Why now:** the substrate stabilized post-m19.1/m36 — runbooks written now describe reality; w7 is the lightest-loaded workstream. **Render parity closing task omitted** — pure infra/ops, no REST/GraphQL/MCP/UI surface.
