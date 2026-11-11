# w8 · m153 — A workspace can create more than one Blueprint from the same `render.yaml`, as on Render

**Worker:** worker8 **Goal:** a second `bex ea blueprints create` for a repo, branch and file that a Blueprint in the workspace already tracks creates a second, independent Blueprint (its own id, sync history and resources), instead of answering `409 BLUEPRINT_CONNECTION_CONFLICT`. **Status:** in progress (t001–t004 and t006–t008 done, code shipped; t005's browser check and t009, the production replay, wait for the next platform deploy)

## Tasks (in order)

| id   | title                                                                  | est | depends_on       |
| ---- | ---------------------------------------------------------------------- | --- | ---------------- |
| t001 | Amend ADR049: Blueprint identity is its id, not repo+branch — **DONE** | 30m | —                |
| t002 | Storage: drop `UNIQUE (tenant_id, repo, branch)` and its three upserts — **DONE** | 60m | t001             |
| t003 | Create: a second Blueprint from the same source is a new Blueprint — **DONE** | 60m | t002             |
| t004 | Push and sync: every Blueprint tracking a repo+branch gets the push — **DONE** | 45m | t002             |
| t005 | Dashboard: create and list more than one Blueprint per repo (implemented and tested; browser check open) | 30m | t003             |
| t006 | Render parity — **DONE**                                               | 20m | t003, t004, t005 |
| t007 | Simplify — **DONE**                                                    | 15m | t006             |
| t008 | Test coverage — **DONE**                                               | 45m | t006             |
| t009 | Closeout (live replay on production)                                   | 20m | t008             |

## Definition of done

1. On production, with the released `bex` CLI and a human login, in one workspace:

   ```text
   bex ea blueprints create --repo https://github.com/render-examples/express-hello-world --name copy-a --auto-sync=false -o json   # → exs-A, plans express-hello-world-<s1>
   bex ea blueprints create --repo https://github.com/render-examples/express-hello-world --name copy-b --auto-sync=false -o json   # → exs-B ≠ exs-A, exit 0, plans express-hello-world-<s2>, s2 ≠ s1
   bex ea blueprints approve exs-A --confirm ; bex ea blueprints approve exs-B --confirm
   ```

   Both services exist, deploy and serve; `GET /v1/blueprints` lists both Blueprints; each Blueprint's detail lists only its own resource.

2. Deleting `exs-B` leaves `exs-A`, its sync history and its service untouched.
3. A push to the tracked branch plans (or auto-syncs) both Blueprints, each against its own resources.
4. `--existing-resources adopt` on a second create follows t001's recorded rule (Render: "Do not manage a particular resource with more than one Blueprint"): it never moves a resource another live Blueprint manages without the existing takeover confirmation.
5. No surface (REST, GraphQL, MCP, dashboard) tells a caller to send a `confirm` phrase merely to create a second Blueprint.
6. Fixtures deleted afterwards.

## Finding (2026-11-11 UTC, `/qa-find-bugs-cli` w8 loop sweep `947f`)

- **Severity:** major — a documented Render workflow (replicating a stack from one Blueprint file) is refused, and the refusal's advice cannot be followed from the CLI.
- **Versions:** released `bex v0.5.0` (Render CLI v2.35.0, `24ac2d60fdb2`); repo HEAD `cb1445664`; production API revision not observable.
- **Context:** `https://api.bex.co/v1/`, workspace `bex-canary`, QA human device login, non-TTY, `-o json`.
- **Repro:**

  ```text
  $ bex ea blueprints create --repo https://github.com/render-examples/express-hello-world --existing-resources adopt --name qa-947f-adopt --auto-sync=false -o json --confirm
  { "data": { "blueprint": { "id": "exs-dbd9mq1ml1cs73dkbfb0", "status": "created", … }, "planningOutcome": "planned", … } }        exit 0

  $ bex ea blueprints create --repo https://github.com/render-examples/express-hello-world --name qa-947f-second -o json --confirm      # default --existing-resources create_new
  Error: received response code 409 (BLUEPRINT_CONNECTION_CONFLICT): blueprint exs-dbd9mq1ml1cs73dkbfb0 ("qa-947f-adopt") already tracks
  https://github.com/render-examples/express-hello-world@main from "render.yaml"; edit the path of that Blueprint to point it at another
  file, or retry with confirm="takeover blueprint exs-dbd9mq1ml1cs73dkbfb0" to replace it                                                exit 1
  ```

  The same 409 answers a second create with `--existing-resources adopt`.

- **Expected (Render):** "You can create multiple Blueprints from a single YAML file. Each Blueprint creates and manages a completely independent set of resources." and "Render appends a suffix to the name of each new resource to prevent collisions with your existing resources." (render.com/docs/infrastructure-as-code § Replicating, read 2026-11-11). The pinned CLI's default `--existing-resources create_new` exists for exactly this, and bex already mints the per-Blueprint name suffix (w2/m261).
- **Actual:** one Blueprint per workspace per repo+branch. The second create is refused, and the message offers two ways out that are not what the user asked for: repoint the first Blueprint, or replace it. The pinned CLI has no flag that sends `confirm`.
- **Attribution:** server (design recorded in ADR049 § "One blueprint per repo+branch", w4/m125, 2026-09-21). That section does not weigh Render's Replicating documentation, and ADR018 records no signed-off divergence, so by the repository's parity rule the ADR is amended (t001), not kept.
- **Root cause:**
  - `lego/backend/internal/store/migrations/0016_blueprints.up.sql:14` — `UNIQUE (tenant_id, repo, branch)`.
  - Upserts on that key: `store/blueprints.go:129`, `store/blueprint_lifecycle.go:261`, `store/blueprint_plans.go:98`.
  - The planned create looks the row up by repo+branch and refuses: `apps/blueprint_onboarding.go:218,233`; `apps/blueprint.go:211-244` (`errBlueprintAlreadyConnected`), `apps/blueprint.go:261-275`.
  - Other readers of "the Blueprint for this repo+branch": `apps/blueprint_ownership.go:579`, `apps/blueprint.go:1693`, and the interface at `apps/blueprint.go:52`.
- **Related observation (covered by t002):** a deleted Blueprint's row is reused by the next create for the same repo+branch. The Blueprint deleted in sweep `2a35` (`DELETE /v1/blueprints/exs-dbd9mq1ml1cs73dkbfb0` → 204, list empty) came back under the same id for a new create 40 minutes later, and that id's embedded timestamp is weeks old. With identity by id, a new Blueprint gets a new id.
- **Blast radius:** REST `POST /v1/blueprints` (planned create) and the legacy create/sync, GraphQL `createPlannedBlueprint`/`createBlueprint`, MCP `create_blueprint`, dashboard connect flow, push-triggered auto-sync, and the resource-claim checks (`BLUEPRINT_RESOURCE_CONFLICT`), which stay as they are: two Blueprints still may not manage one resource.
- **Unverified:** Render's behavior was read from its documentation, not exercised on a Render account; what Render does when a second Blueprint is created in adopt mode against resources the first one manages is undocumented beyond "Do not manage a particular resource with more than one Blueprint". Push fan-out (DoD 3) was not exercised (no push access to the fixture repo; use an owned test repo).
- **Dedupe:** `.pm` has no item for multiple Blueprints per file. w4/m125 (done) introduced the refusal; w2/m261 (done) shipped `create_new` suffixes; w2/m256 t007 (production grading of `ea blueprints`) is still open elsewhere and does not cover this.
- **Cleanup:** both Blueprints of this sweep were deleted (204) and the list is empty; no service was created from them.

## Decisions (t001, 2026-11-11 UTC), recorded in ADR049 § "A Blueprint is its id" and the ADR018 Blueprint row

Render's page was read again on 2026-11-11 (render.com/docs/infrastructure-as-code): "You can create multiple Blueprints from a single YAML file. Each Blueprint creates and manages a completely independent set of resources."; "The Blueprint creation flow displays a notice if your new Blueprint matches existing resources in your workspace"; "Render appends a suffix to the name of each new resource to prevent collisions with your existing resources"; and "Do not manage a particular resource with more than one Blueprint." The page does not say what an adopt-mode create does to a resource another Blueprint manages, and it does not cover deleting a Blueprint.

1. **Identity is the id.** Migration `0229_blueprints_per_source` drops `UNIQUE (tenant_id, repo, branch)` and keeps a plain index.
2. **Every create is a new Blueprint**, on Render's create and on bex's one-step create. `BLUEPRINT_CONNECTION_CONFLICT` and the connection takeover phrase are retired.
3. **A deleted Blueprint is not revived** (the related observation in the finding). The row stays disconnected; the new Blueprint has a new id and its own history. The reconnection note is no longer written.
4. **Adopt on a second create (Definition of done 4):** the create succeeds as a new Blueprint. It never moves a resource another live Blueprint manages: Render's create plans a `failed` sync whose notice names the resource and its manager, and a failed plan cannot be approved; the one-step create answers `BLUEPRINT_RESOURCE_CONFLICT` and proceeds only with that code's takeover phrase.
5. **The retry after a failed apply is kept**, on the one-step create only, and is told apart by state: the newest live row for the same repo, branch, file and mode that is applying or failed, has no successful run and was never planned is continued (same id, suffix, name and auto-sync setting; only the manifest is written); anything else is a new Blueprint. The check and the write hold the row. A second create while the first is applying answers `BLUEPRINT_SYNC_BUSY`. A refused takeover is now answered before admission, so it leaves no Blueprint behind.
6. **The one-step create takes `existingResources` and `resourceSuffix`**; `adopt` stays its default. The preview reports `matchedResources` and reviews in the mode it is given; the dashboard uses `create_new` when there are matches, which is Render's creation flow.
7. **Deploy-time registration** refreshes the oldest live unsuffixed Blueprint of the source, under a per-source lock.
8. **Push fan-out (t004)** needed no new mechanism: the webhook queues one intent per `(delivery, blueprint)` and the worker syncs each under its own claim (w8/m38). `GetBlueprintByRepo` had six callers, all removed: `blueprintConnectionConflict` (`apps/blueprint.go`), the planned create's two checks (`apps/blueprint_onboarding.go`), `previewOwnershipConflicts` (`apps/blueprint_ownership.go`, now given the Blueprint's id), `triggerBlueprintSync` (test-only since w8/m38, deleted), and the store method with its interface entry.

**Follow-ups filed:** w8/250 (the one-step create does not enforce unique Blueprint names) and w8/251 (a failed adopt plan's notice suggests a `confirm` phrase Render's create and approve cannot carry).

## Source + Goal linkage

- **Source:** `/qa-find-bugs-cli` w8 loop, sweep `947f`, 2026-11-11 UTC (finding above).
- **Goal linkage:** Render parity for Blueprints (ADR008 § Render parity first, ADR049): replicating an environment from one file is the documented way to stand up a second copy of a stack.
- **Expected outcome:** `ea blueprints create` behaves as on Render for a repeated source; the conflict code remains only where a resource would change owner.
- **Why now:** `bex v0.5.0` ships `ea blueprints create` with `create_new` as its default, which promises exactly the behavior the server refuses.
- **Render parity included:** the change is on REST/GraphQL/MCP create and the dashboard.
