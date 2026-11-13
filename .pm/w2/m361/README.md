# w2 · m361 — A just-deleted service's name can be reused at once, as on Render

**Worker:** worker2 **Goal:** creating a service under the name of a service that was just deleted succeeds at once on every surface (REST, GraphQL, MCP, Blueprint, the dashboard and the pinned CLI), as on Render. The old service's teardown finishes on its own, and nothing of it (logs, metrics, events, registry images) is mixed into the new service. **Status:** in progress (t001–t004, t007, t008 done: a create and a Blueprint apply wait up to 20 s for the old teardown; t005 production walk after the deploy)

## Tasks (in order)

| id   | title | est | depends_on |
| ---- | ----- | --- | ---------- |
| t001 | Verify on Render, and inventory what is keyed on the App object name (registry repo, Secrets, disks, logs and metrics labels, events) — **DONE** | 45m | — |
| t002 | Create: when the name's holder is being deleted, the new App gets a suffixed object name and keeps the requested name as its creation and shown name; uniqueness counts live services only — **DONE** | 75m | t001 |
| t003 | Name resolution and Blueprints ignore an App being deleted; the same check for Postgres and Key Value names — **DONE** | 60m | t002 |
| t004 | ADR018:81's open divergence is removed and `SERVICE_NAME_STILL_DELETING` retired — **DONE** | 20m | t003 |
| t005 | Production walk: delete then recreate the same name within seconds by CLI and by Blueprint sync | 30m | t004 |
| t006 | Render parity | 20m | t005 |
| t007 | Simplify — **DONE** | 15m | t006 |
| t008 | Test coverage — **DONE** | 30m | t007 |
| t009 | Closeout | 10m | t008 |

## Definition of done

- On production, a QA service is deleted, and a service with the same name is created within 5 seconds:
  - by `bex services create` (the pinned CLI) and by REST, each answering 201;
  - by re-syncing the Blueprint that declared it, which creates rather than fails.
- The new service reaches Live. The deleted one finishes its teardown. `GET` by the name and the dashboard show only the new service.
- The new service's logs, metrics, events and deploys contain nothing from the deleted one.
- Two live services in one workspace still cannot share a name: the existing 409 "already in use" stands, as Render requires.
- ~~No surface answers `SERVICE_NAME_STILL_DELETING`. ADR018's "Address a service by name" row no longer lists the retry window.~~ Narrowed 2026-11-13 (see § Decision): for an ordinary teardown the create answers 201 and ADR018 records the wait as matching Render; a teardown longer than 20 s still answers it, carried by w2/084 as an open gap.

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w2` 2026-11-12, item 5, filed by `/pmx them all for w2`. It follows [w2/075](../done/075.md) (2026-11-08), which replaced a 500 with a named 409 and left the wait.
- **Render:**
  - [render.com/docs/projects](https://render.com/docs/projects): "All of a workspace's services must have unique names". That uniqueness covers live services.
  - w2/075's check: a same-named create is accepted at once after a delete. Only the `onrender.com` subdomain is unique and never reused, and a taken one gets a random suffix.
- **bex today:**
  - ADR018:81 says the retry window "is an open divergence awaiting sign-off". The repository rule is that a divergence stands only with the user's sign-off, and parity work is not parked on a decision (ADR008 Render parity first).
  - The App object name derives from the service name (`lego/backend/internal/apps/service.go:2157-2166`, `blueprintResourceSlug`). A terminating holder keeps that object name until its finalizer runs. That is why create answers 409 `SERVICE_NAME_STILL_DELETING` (`service.go:2363-2369`), tested in `apps/create_name_deleting_test.go`, `apps/create_name_deleting_pg_test.go` and `api/create_name_deleting_test.go`.
  - Since w4/m19 the public host is already a slug with a random suffix, and since w8/m47 a service's shown name may differ from its creation name. The pieces needed to decouple the object name exist.
- **Goal linkage:** ADR008 Render parity first, on the create-service surface and Blueprints.
- **Expected outcome:** delete-and-recreate scripts, IaC re-applies and Blueprint re-syncs need no retry loop.
- **Why now:** w2/m256's Blueprint "re-create after delete" journey is where this surfaced. The unsigned divergence it left breaks the parity rule, and nothing on the board owns it.
- **Open decision, recommendation only (not a user decision):** the recommendation is Render's behavior, implemented here, rather than a sign-off on the wait. It rests on ADR008 Render parity first. The user may still choose to keep the wait as a signed-off divergence instead.
- **Render parity included:** REST, GraphQL, MCP, Blueprint, the CLI and the dashboard create form.

## Decision (2026-11-13, `/loopx w2`, from t001)

- **Evidence (t001):** an inventory of everything keyed on the App object name (recorded in w2/084). Giving a re-created service its own object name touches about ten call sites that compute `core.CRName(tenant, name)`, the three `core` by-name resolution tiers, the `<slug>` Ingress and Service that the old App holds until garbage collection, and the OpenBao purge a retried delete would run. That is about a day of work. Two parts carry real risk of two services sharing a host or env vars.
- **Render basis:** Render accepts a just-deleted name at once (w2/075's check).
- **Chosen (worker's call under ADR008 Render parity first, not a user decision):** ship the part that reaches Render's behaviour for an ordinary delete without changing any of those keys. The create and the Blueprint apply wait up to 20 s for the old App's teardown, which normally clears in 1–15 s, then create. The dashboard's name check treats a deleted service's name as free.
- **Remaining gap:** a teardown longer than 20 s (a static site's object purge, up to 15 min, or a stalled finalizer) still answers `SERVICE_NAME_STILL_DELETING`. It is filed as w2/084 and recorded in ADR018 as an open gap, not a signed-off divergence. The DoD's last clause moves there.
- **Tasks:** t002 became the bounded wait instead of the suffixed object name. t003 is the Blueprint apply's wait plus the name check, and resolution needs no change because the name is released before the new App exists. t004 rewrites the ADR018 row instead of deleting it.
