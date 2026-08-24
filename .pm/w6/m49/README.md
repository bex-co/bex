# w6 · m49 — Duplicate-name creation errors discard the backend's specific conflict reason (extend w4/m19's services-only fix)

**Worker:** worker6 **Goal:** every resource-creation flow that enforces a workspace-unique name gives the user the backend's actual, specific conflict reason on a duplicate — matching the standard `w4/m19` already built and proved for services — instead of a generic "Please try again" toast that tells the user nothing and invites a retry that can never succeed. **Status:** todo

## Tasks (in order)

| id   | title                                                                                                       | est | depends_on                          |
| ---- | ------------------------------------------------------------------------------------------------------------- | --- | ------------------------------------ |
| t001 | Design + implement a stable, backend-copy-independent conflict signal shared by every create-\* consumer      | 45m | —                                    |
| t002 | Key Value: surface the conflict reason instead of the generic toast                                            | 20m | t001                                 |
| t003 | Postgres: surface the conflict reason instead of the generic toast                                             | 20m | t001                                 |
| t004 | Project: surface the conflict reason; decide whether the backend message needs the attempted name added        | 25m | t001                                 |
| t005 | Environments: confirm live whether duplicate names are rejected the same way, then fix if so                   | 30m | t001                                 |
| t006 | API keys / registry credentials: confirm live whether name uniqueness is enforced at all; fix only if it is    | 30m | t001                                 |
| t007 | Render parity across REST/GraphQL/MCP/UI                                                                        | 30m | [t002, t003, t004, t005, t006]       |
| t008 | Simplify the touched code                                                                                       | 25m | t007                                 |
| t009 | Test coverage for the fixed behaviors + regression tests for the already-correct control cases                 | 40m | t007                                 |
| t010 | Closeout                                                                                                        | 10m | t009                                 |

## Definition of done

Every bullet independently live-verifiable against production (or dev-6) while signed in:

- Creating a second Key Value instance with a name already used in the workspace shows the backend's specific conflict reason to the user (not "Please try again") — reproduced via the dashboard UI, not just the API.
- Same for Postgres.
- Same for Project — and the implementation decision on whether the backend message needed the attempted name added (today's project conflict message omits it) is recorded, not left implicit.
- Environments confirmed live (a real duplicate-name create attempt in the UI) and fixed if it exhibits the same swallow; if the live check finds different behavior than predicted, that is recorded rather than assumed away.
- API keys and registry credentials: live-confirmed whether a duplicate name is even rejected server-side; fixed only if a specific rejection reason exists to surface, otherwise closed as not-applicable with the evidence attached to t006.
- The chosen mechanism (structured error code vs. message matching — decided in t001) is applied consistently across every hook touched in this milestone, not ad-hoc per type.
- A regression test confirms `use-create-service.ts`'s existing inline-conflict-with-suggested-name behavior and `use-create-workspace.ts`'s passthrough-error behavior both still work unchanged after this milestone's changes — they are the control cases and must not regress.
- `docs/ADR018-render-parity.md`'s create-surface conflict-handling note is updated to record that the parity `w4/m19` established for services now holds consistently across every resource type that enforces name uniqueness (or explicitly lists which types don't apply, and why).
- `cd lego/backend && go test ./...` + `make lint` green (if the backend gains a structured error code); `dashboard/`: `yarn typecheck && yarn lint && yarn test` green.

## Source + Goal linkage

- **Source:** live `/qa-find-bugs` hunt of `https://dashboard.bex.co`, 2026-08-23, signed in as the QA user, workspace `tea-d98210cbbpdc73dcrkvg` — an adversarial-input testing angle (duplicate-name creation) that prior iterations of this hunt had not covered. Evidence is the raw GraphQL request/response pairs below, run from inside the authenticated page (`page.evaluate`/`fetch(..., {credentials:'include'})`), each reproducible against production; every `qa-20260823-*` resource created to obtain them was deleted immediately after and the Overview page verified back to its pre-hunt baseline.

  1. **Key Value** — created `qa-20260823-duptest` (`red-da5nu9upqvpc73ebkcvg`), then repeated the same name:
     `mutation { createKeyValue(name: "qa-20260823-duptest", plan: "free") { id name } }` →
     `{"data":{"createKeyValue":null},"errors":[{"message":"conflict: a key-value store named \"qa-20260823-duptest\" already exists in this workspace","path":["createKeyValue"]}]}`.
     The dashboard UI's actual toast for the identical action: **"Couldn't create qa-20260823-duptest. Please try again."** — the specific reason above is discarded.
     Root cause: `dashboard/src/features/keyvalue/hooks/use-create-key-value.ts:81-89` — the `catch` block extracts `graphQLErrorMessage(err)` into `msg`, but only branches on `msg.toLowerCase().includes("workspace is limited")` (the unrelated cap-limit case); every other error, including this conflict, falls through to `toast.error(t("keyvalue.createError", { name: input.name }))` (locale string `dashboard/src/features/keyvalue/locales/en.ts:227-230`: `"Couldn't create {name}. Please try again."`). Backend message source: `lego/backend/internal/keyvalue/service.go:352`.

  2. **Postgres** — created `qa-20260823-dbdup` (`dpg-da5nv66pqvpc73ebkd40`), repeated:
     `mutation { createDatabase(name: "qa-20260823-dbdup", plan: "free") { id name } }` →
     `{"data":{"createDatabase":null},"errors":[{"message":"conflict: a Postgres database named \"qa-20260823-dbdup\" already exists in this workspace","path":["createDatabase"]}]}`.
     Root cause: identical shape, `dashboard/src/features/databases/hooks/use-create-database.ts:84-92` (same `graphQLErrorMessage` → only-cap-limit-branch → generic-toast pattern). Backend message source: `lego/backend/internal/postgres/service.go:451`.

  3. **Project** — created `qa-20260823-projdup` (`prj-da5nv1upqvpc73ebkd2g`), repeated:
     `mutation { createProject(name: "qa-20260823-projdup", ownerId: "tea-d98210cbbpdc73dcrkvg") { id name } }` →
     `{"data":{"createProject":null},"errors":[{"message":"conflict: project: already exists","path":["createProject"]}]}` — a **third**, more generic message shape with no resource name in the text, because projects route through the shared `store.ErrConflict` (`"already exists"`, `lego/backend/internal/store/store.go:56`) generic-conflict path rather than a hand-authored per-type message like keyvalue/postgres.
     Root cause: `dashboard/src/features/projects/hooks/use-create-project.ts:28,41` — bare `catch { toast.error(t("projects.createError", { name })) }`; `graphQLErrorMessage` is never called, so every failure of every kind gets the identical generic toast.

  4. **Environments confirmed live 2026-08-23** (t005): created a duplicate-named Environment in the same project — `use-create-environment.ts:40-41`'s bare `catch {}` shows the generic toast while the direct GraphQL probe returns `"conflict: environment: already exists"`, the identical `store.ErrConflict`-backed shape projects hit. Full detail in `t005.md`'s Live probe result section.
  5. **Still code-confirmed only, not independently live-tested** (t006 must confirm live before fixing): `dashboard/src/features/api-keys/hooks/use-create-api-key.ts:53-54`, `dashboard/src/features/registry-credentials/hooks/use-create-registry-credential.ts:52-53` use the identical bare `catch { toast.error(...) }` pattern with no message extraction at all. Whether either even enforces name uniqueness server-side is unconfirmed — a grep of `lego/backend/internal/apikeys/` and `lego/backend/internal/registrycreds/` found no `ErrConflict`/uniqueness handling at all; if neither enforces uniqueness, there is nothing to fix for those two.

  **Control case, confirmed correct** (verified, not assumed): `dashboard/src/features/services/hooks/use-create-service.ts:125-138` branches on `msg.toLowerCase().includes("already in use")` → `setNameConflict(true)` for dedicated inline treatment, backed by `lego/backend/internal/apps/service.go:1674,1885`'s `"name %q is already in use"` message — this is `w4/m19`'s pattern working exactly as designed and **must not regress**. `dashboard/src/features/workspaces/hooks/use-create-workspace.ts:67-68` takes a different, also-correct approach: it passes `graphQLErrorMessage(err)` straight through to an inline `error` state rather than a toast, so the specific backend reason is never lost even without type-specific branching. Both are this milestone's regression-test targets (t009).

  **Implementation wrinkle for t001**: the backend itself is inconsistent — services say "already in use", keyvalue/postgres say "X named Y already exists in this workspace", projects (and inferably environments) say only "<type>: already exists" with no name, under two different sentinels (`core.ErrConflict` = `"conflict"`, `store.ErrConflict` = `"already exists"`). A frontend fix that just adds another `.includes("already exists")` string check per type would be fragile and keep drifting from backend copy changes. The codebase already has a documented, precedent-setting alternative for exactly this problem: `dashboard/src/common/lib/graphql-error.ts`'s `planLimitExtensions`/`hasGraphQLErrorCode` key on a structured GraphQL `extensions.code` (e.g. `"PLAN_LIMIT"`, `"RATE_LIMITED"`) instead of message text, with an explicit comment on why ("backend copy changes have zero effect on whether the CTA shows"). t001 must evaluate (a) giving `core.ErrConflict`/`store.ErrConflict` a stable `extensions.code` (e.g. `"CONFLICT"`) emitted generically on both GraphQL and REST, matching that precedent, against (b) per-type message matching as services already does — and pick one, not default to (b) by inertia.

- **Blast radius:** 10 `use-create-*.ts` hooks total. 2 already correct (services — full `w4/m19` treatment; workspaces — passthrough), both regression-test targets. 4 confirmed broken live (keyvalue, databases, projects — t002/t003/t004; environments confirmed 2026-08-23 — t005). 2 more share the identical bare-catch code shape and are very likely affected but unconfirmed (api-keys, registry-credentials — t006). 2 use a different, seemingly-adequate mechanism not part of this bug class and out of scope: blueprints' plain generic toast has no name-conflict concept at all (may or may not need one — not investigated); webhooks' `WebhookMutationError`/named-refusal system already renders inline for named refusals.
- **Goal linkage:** directly extends `w4/m19`'s already-proven Render-parity + UX pattern ([docs/ADR018-render-parity.md](../../../docs/ADR018-render-parity.md)) to the rest of the resource-type family; a correctness/trust issue — a misleading error message actively points the user at a retry that cannot succeed — more than a cosmetic one.
- **Expected outcome:** every resource-creation flow that enforces workspace-unique names gives the user an accurate, specific reason on conflict, matching the standard `w4/m19` already set and proved end-to-end for services.
- **Why now:** this is the same bug class `w4/m19` already fixed once — its own Definition of done proved the correct behavior exists and works — it was simply never propagated to the sibling create flows. Low risk (mechanical extension of a known-good, already-shipped pattern), high user-trust payoff (a wrong "please try again" message is actively misleading, not merely unpolished).
- **Render parity task included:** `w4/m19` itself included a Render-parity closing task for this exact error-taxonomy surface (REST/GraphQL/MCP/dashboard); this milestone extends the same surface to more resource types, so the same task class applies (t007).
