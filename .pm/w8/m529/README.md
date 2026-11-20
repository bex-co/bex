# w8 · m529 — A restart or rollback pulls its release's image with that release's own registry credential

**Worker:** worker8 **Goal:** the registry credential travels with the release, as the image does. A Restart reruns the serving release with the credential it went live with, a rollback runs its target with the target's, and a later image switch to another credential, failed or merely saved, never changes what the serving release pulls with. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](t001.md) | Each registry credential materializes under its own per-App Secret, so a switch never rewrites the serving release's | 1h15m | — |
| [t002](t002.md) | A release records its registry binding, and Restart and rollback select it | 45m | t001 |
| [t003](t003.md) | A selected release's pods and image reads use its recorded binding | 1h | t002 |
| [t004](t004.md) | ADR004 and ADR018 — the registry credential is part of the release | 20m | t003 |
| [t005](t005.md) | Render parity | 20m | t004 |
| [t006](t006.md) | Simplify | 15m | t005 |
| [t007](t007.md) | Test coverage | 30m | t005, t006 |
| [t008](t008.md) | Closeout | 45m | t007 |

Total: **8 tasks, about 5h10m**.

## Definition of done

1. **Backend.** A backend test pins that the Restart release config (`targetReleaseConfig` with `restart=true`) carries the serving release's recorded binding: no credential when it went live with none, credential A's Secret when it went live with A, whatever the saved binding is now. The same holds for a rollback (`restart=false`) to a deploy that used another credential or none. A record written before this milestone selects nothing, and the release uses today's saved binding.
2. **Per-credential Secrets.** A test pins that materializing credential B for an App leaves the Secret of credential A, which the serving release names, byte-for-byte unchanged. Materializing A again after its token is rotated refreshes A's Secret. Deleting the App removes every pull Secret it has, the legacy `<app>-registry-pull` included. Deleting a credential no App binds removes its per-App copies.
3. **Operator.** An envtest restarts a served release while the spec binds another credential. The restarted Deployment's pod template names the served release's pull Secret, or none when the release had none, and never `spec.externalRegistryPullSecret`. The pre-deploy step Job and the image metadata read (`externalImageAuthHeader`) use the same binding. With the fix reverted, the test fails.
4. **Production**, after a roll that carries this milestone, in `bex-canary` with the bex CLI and the QA user:
   - A free image web service `docker.io/library/nginx:1.27-alpine` with no credential goes live, and its URL answers 200.
   - `bex services update <srv> --image docker.io/<made-up org>/private-none:v1 --registry-credential <fake rgc-> --confirm` saves, and `bex deploys create <srv> --wait --confirm` fails while the URL keeps answering 200.
   - `bex restart <srv> --confirm` reaches `live` within about a minute on the served nginx digest, and the URL answers 200 throughout.
   - `POST /v1/services/<srv>/rollback` to the first deploy reaches `live` in the same state.
   - The fake credential is made up, never a real token. Every fixture (services and the credential) is deleted by ID, and each delete is confirmed with a 404.

## Source + Goal linkage

- **Source:** [w8/353](../done/353.md), filed by the `cli-qa` routine run 14 (`f71b51ca8`, run `r14-f377`). It reproduced on two fresh fixtures on production on 2026-11-20: after an image switch failed on a bad Docker Hub credential, `bex restart` pulled the served `nginx@sha256:65645c7b…` with the failed deploy's credential and failed with Docker Hub's 401. Clearing the saved binding made the same restart go live in 21 s.
- **Cause (traced):**
  - Every deploy trigger re-materializes the saved credential into one per-App Secret, `<app>-registry-pull` (`lego/types/v1alpha1/secretnames.go:48`, `lego/backend/internal/registrycreds/pullsecret.go:132`, called from `lego/backend/internal/deploys/service.go:944`). The trigger also writes `spec.externalRegistryPullSecret`. The projector sets that field on a save alone (`lego/backend/internal/store/reconciler.go:2986` and `:3039`).
  - Restart and rollback select the target's image, port, shutdown delay and pre-deploy command (`targetReleaseConfig`, `lego/backend/internal/deploys/rollback_config.go:46`). The release record (`ReleaseRecordSpec`, `lego/types/v1alpha1/releasesnapshots.go:101`) has no registry binding.
  - The operator gives every pod it builds `app.Spec.ExternalRegistryPullSecret`, whichever release that pod runs (`imagePullSecrets`, `lego/operator/internal/controller/app_controller.go:2436`). The Deployment, the pre-deploy step Job (`:7631`) and the cron pod (`:6179`) all do this. The image metadata read uses the same field (`externalImageAuthHeader`, `image_network.go:124`).
  - Two failure modes follow. A serving release with no credential is restarted with the new credential, which is the production repro. A serving release with credential A has its Secret's contents overwritten with B by the next trigger, so any re-pull (restart, eviction, node drain, wake on a node without the image) uses B.
- **Goal linkage:** Render parity ([ADR008 § Render parity first](../../../docs/ADR008-vision.md#render-parity-first)). Render's Restart keeps "the exact same Git commit and configuration as the running instance" (w4/m433), and a failed deploy leaves the previous release serving and recoverable. Render rolls back to the target's image tag or digest (`render.com/docs/rollbacks`); t002 checks what it documents about that pull's credential.
- **Expected outcome:** one failed or saved-only image switch with a bad or foreign credential no longer breaks the recovery verb. Restart, rollback, eviction and wake keep pulling the serving release with the credential it went live with.
- **Why now:** this is a major bug on production. The serving instance is one reschedule away from an outage after any failed credential switch, and Restart, the documented recovery, fails until the user clears the binding by hand.
- **Render parity task included:** Restart and rollback are tenant-facing verbs on REST, GraphQL, MCP, the CLI and the dashboard. No field changes, but each surface's outcome changes.
- **Related, not in scope:** [w8/354](../354.md) (the error text for an unknown credential) and [w8/355](../355.md) (a credential create without `ownerId`).
