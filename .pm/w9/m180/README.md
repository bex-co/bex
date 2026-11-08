# w9 · m180 — Build Sources: close the gaps between ADR097 and the code

**Worker:** worker9 **Goal:** Build Sources behave as ADR097 and Render say on every path. A create can link services, a successful build always reaches every auto-deploy linked service, linked image/Docker services get the right port and container policy, and Blueprints and the API agree on names, scope and export. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Create accepts `serviceIds` and `envGroupIds`, as on Render | 45m | — |
| t002 | Durable build fan-out: no linked service misses a build across a bex-api restart | 60m | — |
| t003 | A service linked to a Docker or image source gets the carrier's port mode and container policy | 45m | — |
| t004 | Build source names are unique on every surface; Blueprint scope and export keep project and `fromGroup` | 60m | — |
| t005 | Close the service-vs-carrier quota race | 45m | — |
| t006 | Guard SQL enumerations of `apps` against carriers; fix ADR097 and ADR018 text drift | 30m | — |
| t007 | Render parity: REST, GraphQL, MCP and dashboard against Render's Build Sources | 20m | t001, t002, t003, t004, t005, t006 |
| t008 | Simplify | 15m | t007 |
| t009 | Test coverage | 45m | t007, t008 |
| t010 | Closeout | 10m | t009 |

## Definition of done

On production:

- `POST /v1/build-sources` with `serviceIds` links those services.
- After a build source build, every linked auto-deploy service deploys the new image, including when bex-api restarts during the fan-out (verified by the recovery path in a test and one live build).
- A web service linked to an image source serves on the image's port with the `render-v1` policy.
- A duplicate build source name is refused on REST, GraphQL and MCP.
- Generate Blueprint exports a project-scoped build source under `projects[].buildSources` with its `fromGroup` links, and the export re-plans as a no-op.
- ADR097's Decision text matches the implementation.

## Source + Goal linkage

- **Source:** user request 2026-11-08 (`/pm for ADR097 for w9`), from a read-only audit of [ADR097](../../../docs/ADR097-build-sources.md) against the code at `9217e3b49`:
  - `apps/build_sources.go:162-164` (create refuses `serviceIds`);
  - `store/reconciler.go:1002-1005` (fan-out in a fire-and-forget goroutine);
  - `apps/service.go:2608-2646` (port mode and policy read only the service's own source; the open follow-up from w2/m254 t005);
  - `store/store.go:2118` and `blueprint_build_sources.go:211-213,358` (name uniqueness, root-vs-project scope);
  - `blueprint_generate.go:491-520` (export drops project scope and `fromGroup`);
  - `core/build_source_guard_test.go:66` (guards only `AppList`, not SQL over `apps`);
  - the ADR097 quota residual (`store/namespaces.go:187`).
- **Goal linkage:** [ADR008 § Render parity first](../../../docs/ADR008-vision.md#render-parity-first); hosting reliability.
- **Expected outcome:** monorepo users relying on one build feeding several services never end up with a stale service or a wrong port, and Render clients' create bodies work unchanged.
- **Why now:** w2/m254 closed on its happy path. These gaps surface only on restarts, image sources and Blueprints, which are exactly where users will hit them next.
- **Overlap:** w3/057 already tracks two ADR018 corrections (line 82's unexplained ◐ and line 420's stale `w2/m254` "open"); this milestone does not repeat them. t006 fixes only the "16 operations" count.
- **Parity:** included, because these are user-facing REST/GraphQL/MCP/Blueprint behaviors with Render as the reference.
