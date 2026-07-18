# w5 · m45 — Cross-resource post-create landing contract

**Worker:** worker5 **Goal:** Every resource-creation action exposed by the dashboard ends on an intentional, useful screen: standalone resources navigate from the mutation's immutable returned id to their canonical detail or deploy page, while embedded resources and one-time-secret flows remain in the context required to finish safely. **Status:** DONE 2026-07-17 — exhaustive create-action inventory, immutable-id landings, context/secret retention, browser proof, parity evidence, and all dashboard gates passed.

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Inventory every dashboard create action and define its post-create contract — **DONE** | 45m | — |
| t002 | Correct standalone-resource landing behavior using returned immutable ids — **DONE** | 45m | t001 |
| t003 | Preserve and verify contextual and one-time-secret creation outcomes — **DONE** | 45m | t001 |
| t004 | Run the cross-resource browser walk and record durable evidence — **DONE** | 45m | t002, t003 |
| t005 | Render parity check across create responses and dashboard outcomes (standing) — **DONE** | 30m | t004 |
| t006 | Simplify the milestone's changed code (standing) — **DONE** | 30m | t005 |
| t007 | Add meaningful success, failure, and navigation regression coverage (standing) — **DONE** | 45m | t005 |
| t008 | Closeout (standing) — **DONE** | 15m | t006, t007 |

## Definition of done

`docs/render-artifacts/post-create-landings.md` inventories every create action reachable in the current dashboard and names its returned identity, successful next screen, failure behavior, and Render evidence or deliberate bex rationale. At minimum, it covers all service types, managed Postgres, Key Value, Projects, Environments, Environment Groups, Workspaces, webhooks, API keys, registry credentials, SSH keys, workspace invitations, managed-Postgres users/exports, custom domains, and manual deploys. It also records top-level API resource types without a direct dashboard create action, including Blueprint auto-registration, so missing create surfaces cannot be mistaken for verified redirect behavior.

Every standalone resource that has a detail page navigates from the successful mutation's returned immutable id to the most useful canonical bex route without waiting for provisioning: services go to the returned deploy when present and otherwise the service detail, Postgres and Key Value go to their creating detail pages, and Projects and Environment Groups go to their detail pages. Workspace creation selects the new workspace before returning home because there is no separate workspace detail route. Contextual child resources remain in their owning screen, and one-time secrets remain visible until acknowledged; a blanket redirect must never discard an API key, webhook secret, database-user password, or equivalent credential. Failed mutations never navigate.

Route-level tests pin the success and failure outcomes for each behavioral class, including the ordering between dialog/search-param closure and final navigation. A live desktop and narrow-screen create walk verifies representative standalone, contextual, and one-time-secret paths; temporary fixtures are removed. The dashboard typecheck, lint, and test suite pass, and ADR018 plus the evidence artifact honestly record any residual capability gap as follow-up work.

## Completion record

- **Shipped contract:** `docs/render-artifacts/post-create-landings.md` inventories every dashboard create action and its identity, success destination, failure behavior, and Render evidence or deliberate bex rationale. It includes all service types, Postgres create/PITR/users/exports, Key Value, Projects, Environments, both Environment Group entry points, Workspaces, webhooks, API keys, registry credentials, SSH keys, invitations, custom domains, service/environment contents, manual deploy/restart, and rollback.
- **Implementation:** initial Postgres and PITR now open the returned `dpg-*` detail; URL-owned Postgres/Project dialogs await search cleanup before final navigation; Service/Postgres/Key Value/Project/Environment/rollback hooks reject missing ids rather than inventing or reporting false success. Existing Service, Key Value, Project, Environment Group, Workspace, webhook, API-key, credential, invitation, and child-record outcomes were retained and pinned where already correct.
- **Browser proof:** a real Vite dashboard against the in-memory local-bex fixture passed headless Chrome at 1440×900 and 390×844. Postgres `/databases/dpg-localmrq1igj2` and Key Value `/keyvalue/red-localmrq1ilzi` visibly began in `Creating` and polled to `Available`; custom-domain creation stayed on Service Settings with DNS instructions; API-key mint stayed in the one-time acknowledgement dialog and vanished after **Done**; an injected Postgres failure stayed at `/?new=database` with the entered name intact. The final run had no blocking browser-console errors. Gitignored screenshots remain under `.playwright-mcp/`; both local processes were stopped, discarding all in-memory fixtures.
- **Parity and evidence:** ADR018 links the cross-resource landing audit and records the Service, deploy, rollback, and Postgres outcomes. REST/GraphQL/MCP create-response inspection found the identities required by dashboard destinations; no landing depends on a name-derived id. Blueprint remains auto-registered from deploy and has no direct dashboard create action, so it is documented as a separate capability surface rather than falsely counted as redirect coverage.
- **Simplify review:** retained explicit URL-close sequencing in the two affected dialogs and explicit contextual/secret exceptions. A universal redirect helper was rejected because it would hide materially different security and ownership behavior; fixture response shaping was reduced to direct case-local lookups.
- **Quality gates:** after rebasing onto current `origin/main`, `cd dashboard && yarn typecheck` passed; `yarn lint` passed; the focused suite passed 110/110 tests in 15 files; `yarn test` passed 1526/1526 tests in 244 files; `node --check scripts/local-bex.mjs` and `git diff --check` passed.
- **Accepted drift:** dedicated Blueprint creation/import UI remains separately sized, absent capability work and was not added here. Authentication, updates, deletes, assignments, credential rotations, and external Git installation are mutations but not resource-create outcomes; the source audit records that boundary explicitly.

## Source + Goal linkage

- **Source:** promoted from `w5/025.md`, originally the user's 2026-07-17 request to match Render's successful Postgres redirect to `/d/{dpg-id}`, then explicitly expanded by the user to ensure the milestone covers all resource creation rather than only Postgres.
- **Goal linkage:** pillar 1 / Render compatibility (`docs/ADR008-vision.md`, `docs/ADR018-render-parity.md`) and the dashboard's role as the human-facing client of bex-api. Predictable post-create navigation is part of resource lifecycle parity, not merely route polish.
- **Expected outcome:** users never land back on an unrelated list after creating a top-level resource, never lose a one-time credential to an over-broad redirect, and can rely on one documented and regression-tested post-create policy across resource families.
- **Why now:** the Postgres callback already receives the new `dpg-*` id but discards it, while sibling flows have evolved independently. Fixing that one callback alone would leave the same contract vulnerable elsewhere; the existing create routes and detail pages make an exhaustive audit and regression matrix bounded now.
- **Standing closing tasks:** Render parity is included because this changes user-facing dashboard behavior and validates the immutable ids returned by REST, GraphQL, and MCP create surfaces; Simplify, Test coverage, and Closeout follow it.
- **Explicit exclusions:** this milestone does not add an absent creation capability such as a dedicated Blueprint-create screen, invent detail pages for settings-contained child records, persist one-time secrets for later retrieval, change backend resource semantics, or replace the canonical bex routes with Render aliases already handled by w5/m39. Any missing create surface found by t001 is recorded as separately sized follow-up work.
