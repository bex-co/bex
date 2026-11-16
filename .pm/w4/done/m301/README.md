# w4 · m301 — A registry credential applies only to the services that select it, as on Render

**Worker:** worker4 **Goal:** adding, breaking or deleting a workspace registry credential never changes the pull identity of a service that did not select it, and an unselected credential can be deleted. **Status:** done — t001–t008 done; live DoD replayed on production 2026-11-16

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Stop host auto-matching: a credential applies only to the services that select it — **DONE** | 40m | — |
| [t002](done/t002.md) | One-time migration: turn each existing auto-match into an explicit binding (dry-run count first) — **DONE** | 45m | w4/m301/t001 |
| [t003](done/t003.md) | A refused selected credential names the registry, image and credential (401 and 403 alike) — **DONE** | 30m | w4/m301/t001 |
| [t004](done/t004.md) | Record the binding rule in ADR018 and ADR004 — **DONE** | 15m | w4/m301/t001 |
| [t005](done/t005.md) | Render parity — **DONE** | 15m | w4/m301/t002, w4/m301/t003, w4/m301/t004 |
| [t006](done/t006.md) | Simplify — **DONE** | 10m | w4/m301/t005 |
| [t007](done/t007.md) | Test coverage — **DONE** | 30m | w4/m301/t005 |
| [t008](done/t008.md) | Closeout — **DONE** | 10m | w4/m301/t006, w4/m301/t007 |

Eight tasks, about 3h15m.

## Definition of done

- A service that selected no credential pulls anonymously whatever credentials the workspace holds; adding a docker.io credential with a bad token leaves a public docker.io image service deploying live.
- A credential no service selected deletes (204); one a service selected is still refused while bound.
- Services that relied on the old host match keep their credential (migration 0219; production count 0).
- A selected credential the registry refuses (401 Docker Hub, 403 GHCR) fails the deploy naming the registry, the image and the credential.
- ADR018 records the rule.

## Source + Goal linkage

- **Source:** promoted from [w4/474](../../done/474.md) (live `/qa-find-bugs w4` round 12, 2026-11-04) with [done/476](../../done/476.md)'s second repro; user decision 2026-11-05 (`/unblock w4`): option 1, explicit-only binding plus a one-time migration with a dry-run count first.
- **Goal linkage:** Render parity first (ADR008, ADR018); image deploys (ADR004).
- **Expected outcome:** a stray or wrong credential no longer breaks unrelated public-image deploys, and it can be deleted.
- **Why now:** the bug silently changes production pull identity, and the user unblocked it.
- **Render parity:** included (registry credential semantics on every surface).
- **Dry-run (2026-11-10, read-only on the bex-db replica):** `registry_credentials` holds 0 rows in production, so 0 services in every workspace rely on a host match; migration 0219 changes nothing there. 7 apps carry an explicit clear (`''`) and keep it.

- **2026-11-10 — parked (loopx w4):** the code is shipped; only the live DoD replay remains. Production deploys moved to 19:00/00:00/05:00 UTC plus manual dispatch (`4630a48a7`, 2026-11-09), so this waits for the 2026-11-10 19:00 UTC deploy. Then replay the DoD and close. No user action.

## Post-release probe (QA loop round 170, 2026-11-12)

Pin `e7a56b07e0d7` carries the fix. Fixtures are in w4/599, all deleted afterwards (GET 404).

- **DoD 1 passes.** With a docker.io credential holding a bad token in the workspace, the service that selected none (`qa-20261112-r170-anon`) deployed `docker.io/traefik/whoami:v1.10.1` `live`.
- **DoD 2 passes, with a caveat filed as w4/599.**
  - While a service selected the credential, `DELETE` → `409 REGISTRY_CREDENTIAL_IN_USE`.
  - After that service was deleted, the credential deleted with 204, but only ~25 s later. Until then the 409 still named the deleted service, by its Kubernetes object name.
- **DoD 4 passes.** The bound service failed `update_failed` with "docker.io refused this service's registry credential for the image docker.io/traefik/whoami:v1.10.1 (HTTP 401). Check the credential's username and token, or remove the credential from this service to pull a public image anonymously". It names the registry and the image; the credential appears only as "this service's registry credential", not by name.
- **Not run:** the GHCR 403 case, and DoD 3 (migration 0219; production count 0).


## Unblock — 2026-11-16 (loopx w4, verified evidence)

- **Affected:** w4/m301/t008. **Source:** verified evidence, no user decision needed.
- **Finding:** production's scheduled deploy at 2026-11-15 22:49Z ran on `06cc3b02c`; the fix `232c215da` and the round-342 pin `ac206b76841b` are both ancestors (`git merge-base --is-ancestor <sha> 06cc3b02c` exits 0). The release gate is clear, so t008 was replayed and the milestone closed in the same run.

## Closeout replay — 2026-11-16 01:28–01:33 UTC (production, workspace `bex-canary`)

Fixture: Free image web service `qa-20261115-m301-ngx` (`srv-dbt5ob2jclac73969t50`), `docker.io/library/nginx:1.27-alpine`, first deploy `live` (HTTP 200).

- **DoD 1 passes.** `POST /v1/registrycredentials {"registry":"DOCKER",…,"authToken":"not-a-real-token"}` → `rgc-dbt5pdidsq6c73c3lrug` (`host: docker.io`). The service selects none; `POST …/deploys` (`dep-dbt5pdijclac73969tag`) went `live` with the same `image.sha`, URL 200.
- **DoD 2 passes.** That unselected credential: `DELETE` 204, `GET` 404. A second bad credential `rgc-dbt5q0idsq6c73c3ls4g`, selected via `PATCH /v1/services/{id} {"image":{…,"registryCredentialId":"rgc-…"}}`: `DELETE` → 409 `REGISTRY_CREDENTIAL_IN_USE` naming `qa-20261115-m301-ngx (srv-dbt5ob2jclac73969t50)`.
- **DoD 4 passes, 401 and 403 alike.** Docker Hub: `dep-dbt5q0qdsq6c73c3ls60` → `update_failed` "docker.io refused this service's registry credential for the image docker.io/library/nginx:1.27-alpine (HTTP 401). …"; the previous release kept serving (200). GHCR: bad `GITHUB` credential `rgc-dbt5q52jclac73969tl0` selected for `ghcr.io/stefanprodan/podinfo:6.7.0` → `dep-dbt5q5adsq6c73c3ls8g` `update_failed` "ghcr.io refused this service's registry credential … (HTTP 403). …". As in round 342 the message names the credential as "this service's registry credential" (t003's accepted wording), not by name or id.
- **DoD 3** holds by the 2026-11-10 dry-run (production `registry_credentials` count 0, so migration 0219 changes nothing). **DoD 5:** ADR018 records the rule (t004).
- **Cleanup:** `registryCredentialId: ""` unbind 200; both credentials `DELETE` 204; service `DELETE` 204 → `GET` 404; the workspace credential list is `[]`. QA session logged out.
