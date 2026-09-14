# w4 · m106 — Close the GraphQL scope gap that lets a read-classified root field return env-var and secret-file values

**Worker:** worker4 **Goal:** a token scoped `bex.read` cannot read a secret on any surface — the scope class of a GraphQL document reflects where secrets actually live in the schema, not just which root field was named. **Status:** todo

## Tasks (in order)

| id   | title                                                                            | est | depends_on                 |
| ---- | -------------------------------------------------------------------------------- | --- | -------------------------- |
| t001 | Escalate a document's scope class when a sensitive nested field is selected         | 60m | —                          |
| t002 | Audit every nested field that can reach a secret, and pin the list in the guard    | 50m | w4/m106/t001               |
| t003 | Demonstrate the refusal live with a `bex.read`-only token                           | 30m | w4/m106/t001               |
| t004 | Render parity sweep over the changed surfaces                                      | 30m | w4/m106/t002, w4/m106/t003 |
| t005 | Simplify pass over this milestone's changes                                        | 25m | w4/m106/t004               |
| t006 | Test coverage for the shipped behavior                                             | 40m | w4/m106/t004               |
| t007 | Closeout                                                                           | 15m | w4/m106/t006               |

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

- **The three surfaces agree on the class of a secret read.** `REST GET /v1/services/{id}/env-vars`, `MCP list_env_vars`, and `GQL Query.envVars` are already `core.OpClassSensitive` (`scope_matrix_ops.go:601`, `:399`, `:235`). After this milestone the nested GraphQL path is classified sensitive too — by whatever mechanism t001 picks — rather than inheriting `GQL Query.service`'s `core.OpClassRead` (`:274`).
- **The role gate is unchanged and still enforced.** `secrets/service.go:207-217` asserts `core.RelCanViewSensitive` twice (once through `scope`, then an uncached `AuthorizeAppFresh` so a member revoked inside the positive-TTL window cannot reveal one last value). That is the **other** axis and must keep working exactly as it does — a test proves a non-sensitive **role** is still refused after the scope change.
- **A new nested sensitive field cannot ship unclassified.** The guard test fails if a GraphQL field that resolves through `core.EnvVarsFrom`/`core.SecretFilesFrom` (or any future sensitive reader) is reachable from a root classified below sensitive. Today `TestMintAndSensitiveHeuristics` (`scope_matrix_guard_test.go:158-201`) asserts "env-var value read must be sensitive" — but only over `classifiedOps`' keys, which are root fields, so the nested fields are invisible to it.
- **Keys-only projections stay cheap.** `service.envVarKeys` and `service.secretFileNames` return names with empty values (verified live: `{"key":"QA_SENTINEL","value":""}` and `{"content":"","name":"qa-probe.txt"}`) and must **not** be escalated to sensitive — the dashboard's masked list depends on reading them with an ordinary read, and escalating them would force every service page to demand the sensitive scope.

## Source + Goal linkage

- **Source:** live `/qa-find-bugs` hunt of `https://dashboard.bex.co`, 2026-09-14 pass 25. Throwaway free image-backed service `qa-20260914-sec` (`srv-dajqf6ogsm7s73f64210`) with a sentinel env var and secret file, created and deleted inside the run; every probe above is a re-runnable request with its complete response.
- **Goal linkage:** `docs/ADR012-auth.md` §7 and the `bex.read`/`bex.write`/`bex.sensitive` vocabulary `w2/m78` introduced; `docs/ADR006-bex-api.md`'s one-core/three-surfaces rule, which this breaks in the direction that matters most.
- **Expected outcome:** a scope-limited token — the thing a user hands to CI, an agent, or a third-party integration — cannot be used to exfiltrate every env var and secret file in the workspace through a query that looks like an ordinary service read.
- **Why now:** `w4/m105` just made minted API keys request `bex.read`/`bex.write`/`bex.sensitive` from discovery, so scope-limited tokens are about to become the normal case rather than a theoretical one. The gap is cheap to close now and grows teeth the moment users start issuing read-only tokens.
- **Render parity task included:** yes — the fix changes how a GraphQL document is classified, which is a user-visible authorization contract on one of the three surfaces.

## Severity, stated precisely

**Major, not critical, and the distinction is load-bearing:** this is a *least-privilege* failure, not a cross-tenant leak. The caller must still hold the `can_view_sensitive` **role** in that workspace — `secrets/service.go:207-217` enforces it on exactly this path, and a viewer or developer is refused regardless of scope. What fails is the second axis: a workspace admin who deliberately issues a `bex.read` token (to CI, to an agent) gets a token that can still read every secret through GraphQL, while the same token is correctly refused the same data on REST and MCP.

## Dedupe

- **`w2/m78` (done)** built the scope matrix and its `t003` states the GraphQL check is enforced "per top-level field" — so root-only granularity is a **recorded implementation choice**, not an accident. What is *not* recorded anywhere is that two nested fields under a read-classified root return secrets, or that this defeats the sibling guard test's own stated rule. This milestone revisits that interaction; it does not re-litigate the matrix's design.
- **`w2/m84` (done)** added the authorization-decision audit; unrelated to classification granularity.
- **`w4/m105` (done)** is the reason scope-limited tokens matter now, and `w4/076` records that pre-m105 keys carry an empty Hydra scope and must be reminted. Not a duplicate — m105 is about *minting* scopes, this is about *enforcing* them.
- `grep -ril "scope_matrix\|OpClassSensitive\|collectGraphQLTopLevel" .pm --include="*.md"` → 12 files, all the above plus `w5/done/m92` (a different surface) and `w7/done/037`. No open item covers this. `.pm/DO_NOT_DO.md` has no matching anti-goal.
- **Not already fixed on `main`:** `scope_matrix.go:217-246` (`collectGraphQLTopLevel`) still records `GQL <kind>.<name>` for each top-level field and never descends into that field's own selection set; `scope_matrix_ops.go:274` still classifies `GQL Query.service` as `core.OpClassRead`.

## Verified this run, and not claimed

- **Measured live:** both nested fields returning plaintext through `Query.service`; `envVarKeys`/`secretFileNames` masking their values; REST `GET /v1/services/{id}/env-vars` and MCP `list_env_vars` both returning plaintext (correctly — both are classified sensitive).
- **Read from code, not demonstrated:** that a `bex.read`-only token is actually admitted by the gate on this path. Demonstrating it needs a scope-limited token, and on today's production that cannot be minted — `w4/076` records that pre-m105 clients have an empty Hydra scope and requesting `scope=` returns `invalid_scope`, and m105's fix is merged but **not yet deployed** (`w4/078`). **t003 exists precisely to close this gap**, and until it does, the milestone rests on the classification mechanism rather than an observed refusal-bypass.
- **The `Query`-side audit is now done (pass 26) and the blast radius is exactly these two fields.** A depth-4 walk of the whole introspected schema (150 object types) from every `Query` root produced 37 secret-shaped root→leaf paths; every one under a **read** root was probed live, and all of them mask except `service.envVar.value` and `service.secretFile.content`. The strongest controls are the ones with the identical shape that get it right: `envGroup { envVars { value } secretFiles { content } }` and `webhookEndpoints { secret }` are all read-classified roots and all return `""`. Full table, method, and the method's limits are in t002.
- **Still not examined:** whether any *mutation* has the same nesting shape (the collector treats Query and Mutation identically), whether a sensitive field with an unexpected name evades a name-based sweep, and `diskSnapshots.snapshotKey` (an HMAC reference rather than contents, per `lego/backend/CLAUDE.md` — confirm, don't assume). t002 owns all three.
