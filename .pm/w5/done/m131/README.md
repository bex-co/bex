# w5 · m131 — A files write that loses a race cannot drop the mount the winning write set

**Worker:** worker5 **Goal:** The App's reference to its `<service>-files` Secret always ends up where the latest files write leaves it, in either race order, so a pod mounts exactly the files the store and the Secret hold. **Status:** done

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | `rollout.Tracker` gains a locked patch that re-reads the App and re-runs the mutate on a conflict — **DONE** | 30m | — |
| t002 | The single-key secrets verbs patch the App under that lock — **DONE** | 20m | t001 |
| t003 | The batch's App patch re-reads and re-projects on a conflict instead of compensating — **DONE** | 45m | t001 |
| t004 | Regression tests for both race orders and for one deploy row per retried write — **DONE** | 40m | t002, t003 |
| t008 | Env-group link and unlink patch the App under the same lock — **DONE** | 40m | t001 |
| t005 | Simplify — **DONE** | 15m | t004, t008 |
| t006 | Test coverage — **DONE** | 20m | t005 |
| t007 | Closeout — **DONE** | 10m | t006 |

## Definition of done

- **Lost removal:** a deploying batch empties the files map, projects the empty Secret at revision 2 and stalls before its App patch. `SetSecretFile(g)` then commits and projects `{g}` at revision 3 and patches the reference in. When the stalled batch's patch lands, the App still references `<service>-files`.
- **Lost add (the reverse order):** `SetSecretFile(g)` reads the App with the reference present and stalls before its patch. An emptying batch then removes the reference. When `SetSecretFile`'s patch lands, the App references the Secret again, which holds `{g}`.
- **One deploy row:** a write retried after a conflict opens one deploy row, not two.
- Every other `Tracker.Patch` caller is unchanged.
- The secrets, rollout and apps suites pass, and `make lint` is clean.

## Source + Goal linkage

- **Source:** promoted from inbox `w5/117` (found by w5/106's quality review, 2026-10-07). Its first fix, an optimistic lock on the secrets writers, was checked during triage on 2026-10-07:
  - Every files write changes the App: the single-key verbs and a deploying batch bump `restartedAt`, and a save-only batch stamps a fresh `AnnotationSavedConfigRevision`. So the lock orders every pair of files writers.
  - A writer that loses re-reads the App. It also re-runs its projection, which is idempotent per revision and reports `Superseded`/`HoldsData` from the Secret's current state, so it decides the reference again from what the winner left.
  - The env reference is only ever set, never cleared, so it has no such race.
- **Goal linkage:** secrets parity with Render (ADR013). A secret file that is set and stored must mount.
- **Expected outcome:** no ordering of concurrent files writes leaves the store and the Secret holding files the pod does not mount until the next files write.
- **Why now:** w5/106 ordered the Secret's side of these races. This closes the App reference's side, the last half of the same bug.
- **Render parity omitted:** no REST, GraphQL, MCP or UI surface changes; this is internal write ordering.

## Result (2026-10-07)

- **Locked App patches.** `rollout.PatchAppLocked` patches with the resourceVersion the App was read at. On a conflict it re-reads the App and runs the mutate again, and after five tries it answers 409 `CONFLICT` rather than a raw Kubernetes conflict, which would surface as an internal error. `Tracker.PatchLocked` adds the release stamp and opens one deploy row for the patch that lands. Writers using them:
  - the single-key secrets verbs, through `rollApp`;
  - the batch, which uses `rollApp` with a mutate that sets the env reference, re-projects files, and stages or activates;
  - env-group link and unlink; the auto-deploy-gated path uses the plain locked patch, which stamps no release.
- **The files reference follows the latest files write in every order.** Every files write changes the App: a `restartedAt` bump, or a save-only write's fresh saved-config annotation. So the lock orders every pair of writers, and a loser's re-run projection reports what the Secret holds now. The batch projects its env map once, before the patch. Simplify review found that re-projecting a CAS save after a later env write refused a save the store already held.
- **Tests.**
  - Rollout: a lost race is decided again on the winner's App, with one deploy row; persistent contention answers `CONFLICT` after the bound.
  - Secrets, both race orders:
    - an emptying batch that stalls while `SetSecretFile` lands keeps the reference, and its retried patch lands (`RolledOut`);
    - an add whose read predates an emptying batch restores it.
  - A batch that keeps losing compensates and answers 409.
  - A CAS save that loses to an env write still lands.
  - Env groups: a link that loses keeps the service's own files reference, on a rolling service and an auto-deploy-off one.
  - Seven mutants fail these tests: each writer unlocked, the batch compensating on its first conflict, and the CAS re-projection.
  - The secrets, rollout, envgroups, apps, api, deploys and cmd suites pass, and `make lint` is clean.
  - `patchCountingClient` now counts patches that land. One delete test had counted a conflicted attempt the lock now rejects; no other assertion changed meaning.
- **Docs:** ADR013's last open projection window (w5/117) is recorded as closed.
- **Render parity omitted:** no surface changed. A write that keeps losing the App race now answers 409 `CONFLICT` where it used to answer 500.
