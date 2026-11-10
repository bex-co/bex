# w4 · m301 — A registry credential applies only to the services that select it, as on Render

**Worker:** worker4 **Goal:** adding, breaking or deleting a workspace registry credential never changes the pull identity of a service that did not select it, and an unselected credential can be deleted. **Status:** blocked — t001–t007 done (shipped); t008 live replay waits for the 19:00 UTC production deploy

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
| [t008](t008.md) | Closeout | 10m | w4/m301/t006, w4/m301/t007 |

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
