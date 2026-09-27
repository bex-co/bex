# w2 · m94 — Linked environment groups: precedence, auto-deploy, and quota parity

**Worker:** worker2 **Goal:** a linked environment group behaves the way Render documents and the way bex's own Environment page claims. A Render-shaped group write opens a deploy only on linked services with auto-deploy on. A service's own secret file always beats a linked group's file of the same name, the rule env vars already follow. When two linked groups define the same key or file, the page shows which one the service runs. A group already over the secret-map quota can shrink through a batch patch. Every rule is written down. **Status:** t001–t008 done; live re-probe 2026-09-27 (pass 225): bullets 1, 3, 4 and 6 pass; bullet 2 passes on behavior but its page-copy half fails (→ new t010); bullet 5 (over-quota shrink) cannot be seeded from outside and rests on t004 tests; t009 closeout waits on t010

## Tasks (in order)

| id   | title                                                                                                                                                       | est | depends_on |
| ---- | ----------------------------------------------------------------------------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Env-group writes, link, unlink and create-with-services open a deploy only on linked services with auto-deploy on — **DONE**                                          | 60m | —          |
| t002 | A service's own secret file always wins over a linked group's file of the same name; the operator mounts the service files Secret last — **DONE**                     | 45m | —          |
| t003 | Expose link order on the service read; the Environment page orders linked groups by precedence and marks shadowed keys and files — **DONE**                           | 50m | t002       |
| t004 | The env-group batch patch reuses the service rule: a map already over quota can shrink but not grow — **DONE**                                                        | 20m | —          |
| t005 | Record the rules in ADR013 and the ADR018 environment-groups row: service wins, then last linked wins, auto-deploy gating — **DONE**                                   | 15m | t001, t003 |
| t006 | Render parity — **DONE**                                                                                                                                               | 20m | t004, t005 |
| t007 | Simplify — **DONE**                                                                                                                                                    | 20m | t006       |
| t008 | Test coverage — **DONE**                                                                                                                                               | 45m | t006       |
| t010 | Env-group link and delete copy says only auto-deploy services redeploy | 20m | t008 |
| t009 | Closeout | 10m | t008, t010 |


## Decisions

Recorded per t001 step 3, plus two the tasks did not anticipate.

- **The Auto-Deploy gate applies to repo-backed services only.** Render's sentence ("every linked service that has autodeploys enabled") reads unconditional, but bex defaults `spec.autoDeploy` to **false** for an image-backed service purely because there is no branch to watch (`apps/service.go`: "off for an image-backed one (no repo to rebuild from)"), not because its owner declined anything. Gating on the raw boolean would have silently stranded every image-backed service on stale group values forever — a worse bug than the one m94 fixes, and one no DoD bullet would have caught. `autoDeployGated` therefore requires `spec.repo != ""`. Guarded by `TestGroupWriteStillDeploysAnImageBackedService`.
- **`ApplyEnvGroup` (Blueprint sync) inherits the gate.** It already routes through `patchEnvironmentAuthorized` with `SaveModeDeploy`, so it is gated by construction. Keeping it that way is also the right answer: Render's rule is stated about the *linked service's* setting, with no exception for what triggered the group write, and an owner who turned Auto-Deploy off has opted out of releases they did not ask for whichever surface caused one. Source: `render.com/docs/configure-environment-variables` (fetched 2026-09-14).
- **`pendingServiceIds` is NOT added to Render's per-key response objects.** t001 step 4 asked for the skipped ids on "the write result" of every Render-shaped verb. `PUT /v1/env-groups/{id}/env-vars/{key}`, its DELETE, and the secret-file pair return Render's own env-var / secret-file objects (`EnvVarView` even has a custom `MarshalJSON` to match Render byte-for-byte), so adding a bex field there would break the parity those shapes exist to hold. The skip is reported on the two results that are already bex-native supersets — `EnvironmentPatchResult` (`PATCH /contents`, and the shared tail every Render-shaped verb funnels through) and the create-with-services `EnvGroupView` — and is observable everywhere else as the absence of a deploy row. The per-key verbs are still gated; only their *reporting* is unchanged.
- **Link and unlink still patch the spec for a gated service.** The refs must land or the link would not exist. The deploy row and `spec.restartedAt` are what the gate withholds. A link therefore still changes the Deployment's `envFrom` list, which the operator converges on its own schedule; what m94 guarantees is that bex opens no deploy and forces no restart.

## Simplify pass (t007)

- **Applied:** the four Render-shaped roll call sites (create-with-services, link, unlink, value-change) no longer each set `spec.restartedAt` in their own mutate closure — `rollLinked` owns the stamp, which is what makes "gated ⇒ do not stamp" impossible to forget at a call site. `patchWithinQuota` gained two exported wrappers instead of being duplicated in `envgroups`.
- **Declined:** collapsing the six hard-coded `SaveModeDeploy` call sites onto one helper, as t007 suggested. They are not identical once you look — each builds a different `EnvironmentPatch` (bulk vars, one var, one var delete, one file, one file delete, Blueprint apply) and differs in return shape (`EnvVarView`, `SecretFileView`, bare `error`). The shared part is already one function, `patchEnvironmentAuthorized`; a further wrapper would add a layer without removing a branch.

## Render comparison (t006)

Re-read `render.com/docs/configure-environment-variables` on **2026-09-15**; the three sentences m94 depends on (service value always wins, group-vs-group precedence not guaranteed / most recently created, group changes redeploy only autodeploy-enabled linked services) are unchanged from the 2026-09-14 fetch the tasks quote. No new drift to file. The one deliberate divergence (last **linked** wins, not last created) is recorded in `docs/ADR018-render-parity.md` row 116 and `docs/ADR013-secrets.md` § Precedence and redeploy.

## Definition of done

Run every bullet on production (`https://dashboard.bex.co`, workspace `bex`), with throwaway fixtures created and deleted inside the run: a free web service built from `examples/hello-go` (answers `GET /` with `$MESSAGE`, else `OK`) and two environment groups. Each bullet names the state observed at filing time (2026-09-14, w1/091, w1/092, w1/095) so the change is measurable.

- **Auto-deploy off means no deploy.** With the service linked to group A and `PATCH /v1/services/<srv> {"autoDeploy":"no"}` applied, `PUT /v1/env-groups/<A>/env-vars/MESSAGE {"value":"v2"}` returns 200 and **no** new deploy row appears (`GET /v1/services/<srv>/deploys` unchanged, no `deploy_started` event). The group's Secret holds `v2`, and the service still serves the old value until a later deploy. At filing time a `config_change` deploy opened one second after the write and went live with `v2`.
- **Auto-deploy on still deploys.** A second service with `autoDeploy: "yes"` linked to the same group opens a `config_change` deploy on the same write and serves `v2` once live. The write's result names the untouched service (for example under `pendingServiceIDs`), and the group page copy reads that linking or unlinking redeploys linked services that have auto-deploy on.
- **The service's own secret file wins.** A service created with secret file `qa.txt = "from-service"` and a Docker command of `python -m http.server $PORT --directory /etc/secrets` (`examples/hello-python`) serves `from-service` at `/qa.txt` after being linked to a group holding `qa.txt = "from-group"`. At filing time it served `from-group` after the link. Unlinking the group and then removing the service's file falls back to whatever remains, with no stale copy.
- **Group-vs-group collisions are marked.** Link group B, then group A, both defining `MESSAGE`, with no service-level `MESSAGE`. The service serves A's value (last linked wins). On `/services/<srv>/env`, the Linked Environment Groups panel lists A before B, and B's `MESSAGE` carries the shadowed marker (strike-through plus a tooltip naming A). At filing time neither key was marked and A was listed above B by accident, not by rule. The same marker appears for a secret file both groups define.
- **An over-quota group can shrink.** A group seeded over the quota (more than 500 entries or 512 KiB in one map) accepts a `PATCH /v1/env-groups/<id>/contents` that only deletes entries, and refuses one that adds, with the existing refusal sentence. The per-key deletes keep working as the control.
- **The rules are written down.** `docs/ADR013-secrets.md` states the three precedence rules (service over group for env vars and files; last linked wins between groups) and the auto-deploy gate, and the environment-groups row of `docs/ADR018-render-parity.md` records where bex matches Render and where it deliberately diverges (last linked vs Render's most recently created).

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w1` 2026-09-15, proposal 1, absorbing `w1/done/091.md` (group-vs-group collisions unmarked), `w1/done/092.md` (group writes redeploy auto-deploy-off services), `w1/done/095.md` (a group's secret file replaces the service's own), `w1/done/100.md` (over-quota group can't shrink). All four came from the 2026-09-14 live `/qa-find-bugs` hunt and `w1/m147` triage.
- **Goal linkage:** pillar 1 (Render-compatible REST/GraphQL/MCP). Render documents both the service-over-group rule and the "only auto-deploy-enabled linked services redeploy" gate (`render.com/docs/configure-environment-variables`, fetched 2026-09-14). `docs/ADR018-render-parity.md` row 114 marks environment groups ✅ on every surface while missing both divergences. Also pillar 2: the Environment page must state which value the service actually runs.
- **Expected outcome:** no release ships that an owner opted out of by turning auto-deploy off; no credential file is silently swapped at runtime; a collision on the Environment page shows which value wins; a group that outgrew the quota can be brought back under it. Every rule has a written home.
- **Why now:** two of the four are major severity (092, 095) and silent, on a surface Render clients drive through REST with no dashboard in the loop. The quota fix reuses `patchWithinQuota`, which `w1/m147` wrote days ago, so the helper is fresh. The four fixes share `lego/backend/internal/envgroups` and the operator's env/file projection, so one milestone avoids three separate passes over the same code. Independent of the blocked `w1/blocked/m152` (what cancel does), which does not change when a deploy opens.
- **Render parity is included** (t006): REST, GraphQL, MCP and the dashboard all change. The auto-deploy gate touches every Render-shaped group verb; the link order lands on the service read on three adapters; the panel changes the UI.

## Live re-probe (2026-09-27, `/qa-find-bugs` pass 225)

Production, workspace `bex`, deployed `726042a28`, `muse.env` QA credentials. Fixtures:

- groups `qa-20260927-egA` `evg-dascv5psmc7s73cq5opg` (`MESSAGE=v1`, `qa.txt=from-group-A`) and `qa-20260927-egB` `evg-dascv7od0qnc73d7a4r0` (`MESSAGE=from-B`, `qa.txt=from-group-B`);
- repo-backed free Docker services from `bex-co/bex`: `qa-20260927-s1` (`examples/hello-go`, `autoDeploy: no`), `-s2` (`hello-go`, `autoDeploy: yes`), and `-s3` (`examples/hello-python`, `dockerCommand: python -m http.server $PORT --directory /etc/secrets`, own secret file `qa.txt=from-service`).

All were deleted afterwards (`DELETE` 204, then `GET` 404).

- **Auto-deploy off means no deploy — PASS.**
  - Linking A to s1 opened no deploy (deploy count 1 → 1).
  - `PUT /v1/env-groups/<A>/env-vars/MESSAGE {"value":"v2"}` → `200 {"key":"MESSAGE"}`. s1's deploy count stayed at 1, its Events feed after the write was empty, the group read back `v2`, and `https://qa-20260927-s1.onbex.co/` kept serving **`v1`**.
  - Observation matching the Decisions section: s1 *did* start serving `v1` after the link without any deploy row. The link still changes the Deployment's `envFrom`, which Kubernetes rolls out; the gate withholds the deploy row and forced restart, not the ref change.
- **Auto-deploy on still deploys — PASS on behavior; copy FAIL.**
  - The same write opened a `config_change` deploy on s2, which then served `v2`.
  - `PATCH /v1/env-groups/<A>/contents {"saveMode":"deploy", …}` → `affectedServiceIds` [s1, s2, s3], `pendingServiceIds: ["<s1>"]`. That names the untouched service, on the batch response as the Decisions section chose.
  - **Copy:** the group page still reads "Linking or unlinking redeploys every affected service." (`envGroups.servicesDescription`), not the gate-aware sentence t001 step 4 required. → **t010**.
- **The service's own secret file wins — PASS.** s3 served `from-service` at `/qa.txt` before, and still after, group A (holding `qa.txt=from-group-A`) was linked and its link deploy went live. The unlink-then-remove fallback was not exercised.
- **Group-vs-group collisions are marked — PASS.**
  - B was linked to s2, then A. s2 served `v1` (A's value, last linked wins).
  - `/services/<s2>/env` → Linked Environment Groups lists **qa-20260927-egA** before **qa-20260927-egB**.
  - B's `MESSAGE` and `qa.txt` render struck through, each with the title "Overridden by qa-20260927-egA, which is linked later".
- **An over-quota group can shrink — NOT RUN.** A group over 500 entries / 512 KiB cannot be created through the API, because the quota refuses the growth that would seed it. This rests on t004's tests.
- **The rules are written down — PASS.** `docs/ADR013-secrets.md:96-99` states service-over-group (env vars and files), last-linked-wins, and the auto-deploy gate. The ADR018 environment-groups row (line 120) records last-linked-wins as a deliberate divergence from Render's most-recently-created.

