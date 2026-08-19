# w2 · m78 — ADR069 follow-up: operation-level OAuth scope matrix + authorization-decision audit

**Worker:** worker2 **Goal:** move from the round-14 coarse `bex.api` capability scope to a least-privilege operation-level scope vocabulary (read / write / credential-mint classes) enforced at introspection across REST/GraphQL/MCP, advertised at consent/discovery, with every authorization decision audited with client id, subject, audience, and normalized scopes. **Status:** todo

## Tasks (in order)

| id                           | title                                                                                                                                                                             | est | depends_on               |
| ---------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | --- | ------------------------ |
| t001                         | Vocabulary + matrix design: ADR012 §7 amendment — scope classes (read/write/credential-mint riding `AuthorizeMintClass`), `bex.api` as implies-all umbrella, exemptions unchanged   | 45m | —                        |
| t002                         | Classification table: every REST route, GraphQL operation, MCP tool classified, checked in, guard test fails on unclassified additions                                              | 45m | w2/m78/t001              |
| t003                         | Introspection/authz enforcement: token scopes → allowed classes at the auth gate; back-compat (`bex.api` grants all; ADR069 exemptions preserved); consistent 403 refusal           | 60m | w2/m78/t002              |
| t004                         | Consent + discovery: RFC 9728 `scopes_supported` advertises granular scopes; dashboard consent vocabulary intersects/refuses consistently on GET+POST (`OAUTH_API_SCOPE` lockstep)  | 45m | w2/m78/t003              |
| t005                         | Authorization-decision audit: bounded record of client id, subject, audience, normalized scopes per decision — never token material                                                 | 30m | w2/m78/t003              |
| t006 (standing closing task) | Render parity: refusal error shapes identical + Render-dialect across the three surfaces; bex-extension status documented                                                           | 30m | w2/m78/t004, w2/m78/t005 |
| t007 (standing closing task) | Simplify: run /simplify over the changed code                                                                                                                                      | 30m | w2/m78/t006              |
| t008 (standing closing task) | Test coverage: negative-path matrix (read-scoped token attempting write/mint on each surface; legacy tokens unaffected; exemptions hold; dashboard consent paths)                   | 45m | w2/m78/t006              |
| t009 (standing closing task) | Closeout: verify DoD, mark done, move milestone to done/                                                                                                                           | 15m | w2/m78/t008              |

## Definition of done

A third-party client can obtain a read-only-scoped token through ordinary consent; write/mint attempts with it are refused with a consistent Render-dialect 403 on all three surfaces while full-`bex.api` tokens and ADR069's four legitimate caller shapes (compliant third party, platform-marked client, `client_credentials` API key, audience-less device flow) are byte-identically unaffected; a checked-in classification table covers every registered REST route, GraphQL mutation/query, and MCP tool with a guard test failing on unclassified additions; authz decisions carry client/subject/aud/scopes in the audit stream; ADR069's "board item" follow-up line is annotated as owned/closed.

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w2` 2026-08-18 (round 1, item 3); ADR069 finding 1's recorded follow-up (`docs/ADR069-security-review-round14.md` line 114), which says the matrix "belongs on the board, not in a remediation round" — grep confirmed no `.pm` item was ever filed.
- **Goal linkage:** least-privilege for the agent/MCP connection story (pillar 3; ADR012 §7, ADR025) — every third-party MCP client currently receives the user's entire authority.
- **Expected outcome:** read-only agent tokens become possible; future security rounds stop re-reporting the deferral.
- **Why now:** round 14 closed the vulnerability but shipped only the coarse scope; as agent connections are bex's differentiator, scope granularity is a product need, not just hardening.
- **Render parity included (narrow):** scopes are a documented bex extension (Render API keys are all-powerful); the parity check is that refusal error shapes stay Render-dialect and identical across REST/GraphQL/MCP.
