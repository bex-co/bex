# w2 · m365 — A re-created service gets its own App object, so a name is free at once whatever the old teardown takes

**Worker:** worker2 **Goal:** creating a service under the name of one just deleted answers 201 at once, as on Render, even while the old service's teardown runs for minutes (a static site's object purge, a stalled finalizer). The old and new services never share a host, a Service, env vars, logs, metrics or artifacts. **Status:** done (2026-11-14; shipped `18b9e89d5`, walked on production at pin `706a389d5` in t006)

## Tasks (in order)

| id   | title | est | depends_on |
| ---- | ----- | --- | ---------- |
| t001 | ✅ Store the App object name: an `apps.object_name` column, written at create and read wherever a service's App is fetched by `CRName(tenant, name)` today | 2h | — |
| t002 | ✅ A create whose name is held only by an App being deleted gets a suffixed object name at once, from REST, GraphQL, MCP and a Blueprint apply | 75m | t001 |
| t003 | ✅ By-name resolution skips Apps being deleted, and a delete of an App already being deleted does nothing more | 60m | t002 |
| t004 | ✅ The old service's host and slug Service never collide with the new one | 90m | t002 |
| t005 | ✅ `SERVICE_NAME_STILL_DELETING` and the 20 s wait retire; ADR018's open gap closes | 30m | t003, t004 |
| t006 | ✅ Production walk: re-create a static site and a web service while their old teardown is still running | 45m | t005 |
| t007 | ✅ Render parity | 20m | t006 |
| t008 | ✅ Simplify | 15m | t007 |
| t009 | ✅ Test coverage | 45m | t008 |
| t010 | ✅ Closeout | 10m | t009 |

## Definition of done

- On production, a static site and a web service are each deleted. A service with the same name is created while the old App is still terminating, and the create answers 201 within a second. The new service reaches Live and serves its own content on its own host.
- While both exist, the old and new services share no Ingress host, Kubernetes Service, OpenBao env path, registry repository, static prefix, log stream or metric series. A repeated `DELETE` of the old service's `srv-` id does not touch the new service.
- `GET` by name, the services list and the dashboard show only the new service. The old one finishes its teardown on its own.
- Two live services in one workspace still cannot share a name.
- No surface answers `SERVICE_NAME_STILL_DELETING`. ADR018's "Address a service by name" row records immediate reuse with no open gap.

## Source + Goal linkage

- **Source:** promoted from [w2/084](../084.md) (2026-11-13), filed by w2/m361. That milestone's t001 inventory is the task list.
- **Render:** a same-named create is accepted at once after a delete (w2/075's check). Only the `onrender.com` subdomain is unique, and a taken one gets a random suffix. (w2/075 also called it never reused; ADR005 § Accepted residuals, from w5/m137 t007, records that bex mirrors Render by re-minting a bare slug once no live service holds it, and production does so.)
- **bex today:** w2/m361 waits up to 20 s for the old App's teardown. A longer teardown still answers `409 SERVICE_NAME_STILL_DELETING`, an open gap in ADR018.
- **Goal linkage:** ADR008 Render parity first, on the create-service surface and Blueprints.
- **Expected outcome:** delete-and-recreate never waits or retries, static sites included.
- **Why now:** the gap is open, and ADR018 records it as unsigned. The rule is that parity gaps are owned and closed, not parked.
- **Render parity included:** REST, GraphQL, MCP, Blueprint, the CLI and the dashboard create form.
