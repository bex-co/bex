# w5 · m148 — Services see Render's platform environment variables (`RENDER_*`)

**Worker:** worker5 **Goal:** a service on bex sees the `RENDER_*` variables Render documents, so Render's framework guides (Django `ALLOWED_HOSTS`, Rails hosts, commit-stamped builds) work unchanged. **Status:** done

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Pin Render's platform variable set and amend the "narrower set" decision — **DONE** | 40m | — |
| t002 | The operator injects the runtime set, yielding to the service's own values — **DONE** | 1h30m | t001 |
| t003 | Native and Docker builds see the build-phase subset — **DONE** | 1h | t001 |
| t004 | Blueprint and env writes accept a user-set RENDER_* where Render does — **DONE** | 45m | t001 |
| t005 | Envtests and a dev-5 Django app reading RENDER_EXTERNAL_HOSTNAME — **DONE** | 1h | t002, t003 |
| t006 | Simplify — **DONE** | 20m | t004, t005 |
| t007 | Test coverage — **DONE** | 45m | t004, t005, t006 |
| t008 | Closeout — **DONE** | 10m | t007 |

Total: **8 tasks, about 6h10m**.

## Definition of done

- A new web service sees `RENDER_EXTERNAL_URL` equal to its public URL and `RENDER_GIT_COMMIT` equal to the deployed commit; a service's own value wins.
- The pinned set (phase and source per variable) is recorded in ADR018.

## Source + Goal linkage

- **Source:** fifth `/pm-brainstorm for w5` (2026-10-20), all five items chosen by the user the same day after `2163b27c7` made Render parity the top principle (a divergence stands only with ADR018-recorded user sign-off; never park parity on an optional decision); w4/m123 t005 chose a narrower set than Render.
- **Goal linkage:** ADR008 Render parity first; ADR004 runtime env.
- **Expected outcome:** apps that read Render's platform variables migrate without edits.
- **Why now:** the largest silent runtime gap left after m147's `WEB_CONCURRENCY`, and the yielding mechanism is fresh.
- **Render parity:** omitted: runtime/build environment only; Render does not list these in the env editor and no REST/GraphQL/MCP/UI field changes (t004 aligns write-path validation, covered by its own tests).

## Closeout (2026-10-20)

- **Pinned set.** Render's 15 documented variables are recorded in ADR018 (new row above `PORT`) and ADR004. One package, `lego/types/renderenv`, computes the static values for both the operator and bex-api.
- **Runtime.** The operator sets the set on the app container, cron runs and pre-deploy jobs, as platform-owned entries before `PORT`. `RENDER_INSTANCE_ID` is the pod name.
  - `RENDER_GIT_COMMIT` comes from the serving image's OCI revision label. Every BuildKit build stamps it from the clone step's commit, so held and rolled-back releases report their own commit. The read is best effort and never holds a rollout.
  - `RENDER_DISCOVERY_SERVICE` names a headless `<slug>-discovery` Service selecting every instance of a web, private or worker service. The operator never adopts a same-named object it doesn't control.
- **Build.** Native and Dockerfile builds get the build-time subset. `RENDER_GIT_COMMIT` is passed as a build-arg, with `ARG RENDER_GIT_COMMIT` before the native build `RUN`.
- **Blueprints.** `fromService.envVarKey` copies of a sibling's platform variable resolve at apply. The per-deploy commit, the per-instance id and variables the sibling's type lacks are refused by name. The corpus `"RENDER_"` divergence is gone. That entry had been hiding a separate gap, copying a sibling's `sync: false` variable, now filed as w5/170 and tracked in `knownBlueprintGaps`. The corpus still passes (71/170 valid; every refusal is deliberate or filed).
- **Tests:**
  - `types/renderenv/renderenv_test.go`.
  - Controller: `platform_env_test.go`, with unit tests plus envtests for env projection, the discovery Service, the unowned-name skip and cron having no discovery Service.
  - `registry/image_config_test.go` (the revision read ignores port rules and verifies digests).
  - BuildKit script and Dockerfile tests updated.
  - Backend: `blueprint_platform_env_ref_test.go`.
  - The full operator suite and lint pass for the types, operator and backend modules.
- **Deviation from the DoD (t005).** dev-5 reuses the shared cluster's operator, so the Django `ALLOWED_HOSTS` check can't run on dev-5 before this ships. The Deployment env (including `RENDER_EXTERNAL_HOSTNAME`) is proven in envtest instead. Two runtime pieces are verified only by tests, not live: the label stamping (`--opt label:` on the dockerfile frontend) and the commit read-back.
- **Rollout.** There is no creation cutoff (Render sets these on every service), so the operator upgrade rolls every running service once, as a normal rolling update.
