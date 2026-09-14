# w4 · m106 — Close the GraphQL scope gap that lets a read-classified root field return env-var and secret-file values

**Worker:** worker4 **Goal:** a token scoped `bex.read` cannot read a secret on any surface — the scope class of a GraphQL document reflects where secrets actually live in the schema, not just which root field was named. **Status:** done

## Tasks (in order)

| id   | title                                                                            | est | depends_on                 |
| ---- | -------------------------------------------------------------------------------- | --- | -------------------------- |
| t001 | Escalate a document's scope class when a sensitive nested field is selected — **DONE** | 60m | —                          |
| t002 | Audit every nested field that can reach a secret, and pin the list in the guard — **DONE** | 50m | w4/m106/t001               |
| t003 | Demonstrate the refusal live with a `bex.read`-only token — **DONE**               | 30m | w4/m106/t001               |
| t004 | Render parity sweep over the changed surfaces — **DONE**                           | 30m | w4/m106/t002, w4/m106/t003 |
| t005 | Simplify pass over this milestone's changes — **DONE**                             | 25m | w4/m106/t004               |
| t006 | Test coverage for the shipped behavior — **DONE**                                  | 40m | w4/m106/t004               |
| t007 | Closeout — **DONE**                                                                | 15m | w4/m106/t006               |

## Definition of done

- **A `bex.read`-scoped token is refused a secret on GraphQL, exactly as it already is on REST and MCP.** With a token carrying `bex.read` but not `bex.sensitive`:

  ```graphql
  query { service(id: "srv-…") { envVar(key: "ANY_KEY") { value } } }
  query { service(id: "srv-…") { secretFile(name: "any.txt") { content } } }
  ```

  both fail the scope gate. Today both **return the plaintext value** — measured live on 2026-09-14 (pass 25) with a throwaway service:

  ```text
  { service(id:"srv-dajqf6ogsm7s73f64210"){ envVar(key:"QA_SENTINEL"){ key value } } }
    → {"data":{"service":{"envVar":{"key":"QA_SENTINEL","value":"qa-sentinel-value-12345"}}}}
  { service(id:"srv-…"){ secretFile(name:"qa-probe.txt"){ name content } } }
    → {"data":{"service":{"secretFile":{"content":"qa-file-sentinel-67890","name":"qa-probe.txt"}}}}
  ```

- **The three surfaces agree on the class of a secret read.** `REST GET /v1/services/{id}/env-vars`, `MCP list_env_vars`, and `GQL Query.envVars` are already `core.OpClassSensitive`. After this milestone the nested GraphQL path escalates to sensitive too (selection-set walk over `envVar`/`secretFile`) rather than inheriting `GQL Query.service`'s `core.OpClassRead`.
- **The role gate is unchanged and still enforced.** `secrets/service.go` still asserts `core.RelCanViewSensitive` (including uncached `AuthorizeAppFresh`).
- **A new nested sensitive field cannot ship unclassified.** `TestGraphQLSensitiveNestedFieldsPinned` fails if `envVarValueResolve` / `secretFileContentResolve` drift off the registry.
- **Keys-only projections stay cheap at the dispatch matrix.** `service.envVarKeys` and `service.secretFileNames` are not in `graphQLSensitiveNestedFields` (dashboard sessions remain `CapabilityExempt`).

## Shipped mechanism

Option (a): keep the root matrix; walk the selection set and escalate when a registered nested field is selected. Human OAuth `bex.read` refuses nested secrets at dispatch (`TestScopeClassEnforcementReadToken`); write does not imply sensitive (`TestScopeClassEnforcementWriteToken`). Documented in ADR012 §7 and ADR018 (bex extension — Render API keys are unscoped).

**Query-side audit (pass 26):** blast radius is exactly `service.envVar.value` and `service.secretFile.content`. Env-group / webhook nested secrets under read roots already mask. Full table in `done/t002.md`.

**t003 correction:** API keys are `CapabilityExempt` and never hit `RequireOpClass`. Refusal is demonstrated with a non-exempt human OAuth identity carrying only `bex.read` (enforcement tests), not with an API key.

## Source + Goal linkage

- **Source:** live `/qa-find-bugs` hunt of `https://dashboard.bex.co`, 2026-09-14 pass 25 (+ pass 26 audit).
- **Goal linkage:** `docs/ADR012-auth.md` §7; `docs/ADR006-bex-api.md` one-core/three-surfaces.
