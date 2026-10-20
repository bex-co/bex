# CLI sweep 929 — GraphQL variables + introspection (live, 2026-10-20)

Read-only GraphQL probes (baseline read only). No new bug filed.

- The `String!` variable form works end-to-end (`service(id: $id)` resolves the baseline) — sweep 928's refusal was specifically the nonexistent `ID` scalar, not variables support.
- Introspection is enabled (`__type(name:"Mutation")` enumerates fields) — consistent with a public API whose dashboard is a GraphQL client; schema discovery is not treated as sensitive here. Recorded as an observation, not a finding.
