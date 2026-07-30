# w7 · m55 — Cross-workspace negative-authorization matrix

**Worker:** worker7 **Goal:** make any authorization regression that opens cross-workspace reads/writes — including wrong-object (confused-deputy) authorization — a CI failure across REST, GraphQL, and MCP. **Status:** todo

## Tasks (in order)

| id   | title                                                                          | est | depends_on |
| ---- | ------------------------------------------------------------------------------ | --- | ---------- |
| t001 | Two-workspace deny fixture + verb-enumeration harness                          | 90m | —          |
| t002 | REST route matrix with completeness guard (~186 registered routes)             | 60m | t001       |
| t003 | GraphQL + MCP non-member deny matrices                                         | 60m | t001       |
| t004 | Extend the OpenFGA-backed multi-workspace E2E to a per-service sample          | 45m | t002, t003 |
| t005 | Simplify                                                                       | 30m | t004       |
| t006 | Test coverage: seeded-regression (mutation) verification of the matrix itself  | 45m | t004       |
| t007 | Closeout                                                                       | 15m | t006       |

## Definition of done

CI fails whenever any REST route, GraphQL verb, or MCP tool lets a non-member caller read or mutate another workspace's resource — including a verb that authorizes the caller's own workspace instead of the resource's owning workspace (a failure mode the existing deny-all sweep provably cannot catch). Coverage is **enumerated**, not hand-picked: a newly added service verb, REST route, or MCP tool either automatically joins the matrix or fails a completeness guard. The OpenFGA-backed E2E covers a representative route per service against real FGA tuples and runs unconditionally in CI (`backend-test.yml` ephemeral containers). A deliberately seeded authz bypass (t006) demonstrably turns the suite red.

## Source + Goal linkage

- **Source:** promotes `w7/009.md` (investigate-first note, 2026-07-19). The 2026-07-30 coverage sweep confirmed the gap: `TestAuthzGuardsEveryVerb` (`lego/backend/internal/api/server_test.go:1012`) asserts only deny-all behavior per service verb; ~15 hand-written cross-workspace spot tests cover 11 services; only 3 of ~186 REST routes get a real two-workspace deny check (`internal/api/multiworkspace_e2e_test.go:54`); MCP tools have zero dedicated cross-workspace tests; the m30 conformance suite is authz-blind.
- **Goal linkage:** GOAL.md V0 #7 (security review) — the multi-tenant trust boundary is only as strong as the regression guard behind it; the security-regression analog of m30's contract-conformance suite (w7's CI-guard charter).
- **Expected outcome:** an OpenFGA-model or per-handler authz-wiring regression that silently opens cross-workspace access becomes a CI failure instead of a production incident.
- **Why now:** authorization is enforced per-handler (`core.Base.Authorize*` — every new verb re-decides authz), and verbs were added continuously this month (sandboxes, billing); the current guard cannot catch the confused-deputy failure mode at all.
- **Render parity:** omitted — pure test/CI work with no REST/GraphQL/MCP/UI surface change (the m30 precedent).
