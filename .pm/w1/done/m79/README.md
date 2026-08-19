# w1 · m79 — Dashboard GraphQL codegen convergence

**Worker:** worker1 **Goal:** retire the hand-written GraphQL "shadow layer" whose stated premise ("until codegen is next run") is now false — all 212 operations exist in `src/graphql/definitions.ts` — and whose hand-rolled types silently lie (`__typename` omitted while codegen emits it non-optional). Also close the undeclared-dependency build risk it created. **Status:** done

## Tasks (in order)

| id   | title                                                          | est | depends_on |
| ---- | -------------------------------------------------------------- | --- | ---------- |
| t001 | Add the 3 missing operations to `.graphql` files + regenerate — **DONE** | 45m | —          |
| t002 | Reduce shadow `operations.ts` files to re-export shims — **DONE** | 1h  | t001       |
| t003 | Resolve the undeclared `graphql-tag`/typed-document-node deps — **DONE** | 20m | t002       |
| t004 | Simplify — **DONE** | 30m | t002, t003 |
| t005 | Test coverage — **DONE** | 45m | t002, t003 |
| t006 | Closeout — **DONE** | 15m | t005       |

## Definition of done

Every GraphQL operation the dashboard executes is defined in a `.graphql` file and consumed via `@/graphql/definitions` (directly or through a ≤5-line re-export shim like `features/webhooks/api/operations.ts`); no hand-written `TypedDocumentNode` bodies remain; blueprints has a real `blueprints.graphql`; `yarn typecheck`, `yarn test`, and `yarn build` are green with `package.json` declaring (or no longer needing) `graphql-tag` and `@graphql-typed-document-node/core`.

## Source + Goal linkage

- **Source:** 2026-08-19 architectural refactor review §2.3 (ledger artifact: https://claude.ai/code/artifact/fe4af1ce-211f-4109-a541-f0aabd273c73). Evidence: `features/databases/api/operations.ts` (628 lines, 24/25 ops already in definitions), keyvalue 6/6, services 1/1; blueprints has 10 hand-maintained ops and no `.graphql` file; codegen's `nonOptionalTypename: true` (`codegen.ts:40`) makes the shadow types wrong; 8 files import two packages that exist only as devDependency transitives.
- **Goal linkage:** dashboard reliability as the human surface of the Render-parity product; removes a class of silent type lies that will bite the first cache-update/`readFragment` written against the shadow types.
- **Expected outcome:** ~840 lines deleted; one source of truth for operation types; a hoisting/devDep change can no longer break `yarn build`.
- **Why now:** the drift compounds with every new operation, and the fix is mechanical while the two layers still agree at runtime. Render parity omitted: no REST/GraphQL/MCP wire change and no user-visible dashboard behavior change — internal data-layer type plumbing only.
