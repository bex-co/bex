# w7 · m61 — Insider role-ladder deny matrix: every mutating verb × under-privileged role (m55 sibling)

**Worker:** worker7 **Goal:** CI systematically proves that within-workspace roles are enforced per verb — a viewer/billing member is denied every mutating verb, a contributor can operate but not create/delete — completing w7/m55's negative-authorization program on its second (insider) axis **Status:** todo

## Tasks (in order)

| id   | title                                                                                       | est | depends_on |
| ---- | -------------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Pin the role→relation grant table from `model.fga` in a test helper                           | 45m | —          |
| t002 | Role-ladder sweep: every sweepable verb × {viewer, contributor, billing}, completeness-guarded | 60m | t001       |
| t003 | Real-OpenFGA E2E: one verb per relation class per role                                        | 45m | t002       |
| t004 | Simplify pass over the changed code                                                           | 20m | t003       |
| t005 | Test coverage: anti-tautology self-tests — each regression class turns CI red                 | 45m | t003       |
| t006 | Closeout                                                                                      | 10m | t005       |

## Definition of done

- A CI test proves the full mutating-verb × under-privileged-role matrix: for every sweepable verb and each of {viewer, contributor, billing}, the verb denies when the role lacks its relation and allows when it holds it — completeness-guarded via the same wired-service inventory m55 uses, so a new verb cannot ship unswept.
- The role→relation table is pinned against `deploy/gitops/authz/model.fga`: a model edit that changes any role's grants without updating the table turns CI red.
- A real-OpenFGA E2E exercises one verb per relation class per role (contributor operate-allowed/create-denied; billing billing-only; viewer read-only + sensitive-denied).
- Anti-tautology self-tests prove the matrix catches its regression classes (e.g. a verb gated on a weaker relation than its class requires).
- Backend suite + lint green.

## Source + Goal linkage

- **Source:** `/pm-brainstorm more for w7` round 2, 2026-07-30 authz sweep — enforcement mechanism verified COVERED (five roles → distinct FGA relations, `deploy/gitops/authz/model.fga:14-76`; every verb names its relation), but matrix **test coverage** verified PARTIAL: only one real-FGA E2E samples the viewer role against a handful of verbs (`lego/backend/internal/api/multiworkspace_e2e_test.go:209-287`); nothing systematically proves the ladder for contributor/billing, and no guard catches a future verb shipping with a too-weak relation (e.g. a delete gated on `can_operate`).
- **Goal linkage:** enforced authz (ADR012 auth, ADR024 members/roles) — w7/m55 closed the outsider (non-member) axis; this closes the insider (role) axis with the same enumerated-matrix + completeness-guard method.
- **Expected outcome:** a role-privilege regression in any current or future verb turns CI red instead of shipping; the five-role contract in `model.fga` is executable, not just declarative.
- **Why now:** m55 closed 2026-07-30 with its tooling (sweep inventory, completeness guards, real-FGA harness) fresh and directly reusable; every verb shipped meanwhile is unguarded on this axis.
- **Render parity:** omitted — test-only milestone, no REST/GraphQL/MCP/UI surface change.
