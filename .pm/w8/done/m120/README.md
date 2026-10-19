# w8 · m120 — A release held through rollout keeps its own settings

**Worker:** worker8 **Goal:** A release the latest-pending slot holds through its rollout (ADR060 §D1a, Render's Wait policy) rolls out with its own settings: start command, environment, files, health path and replicas. Its release record says so too, so Restart, Rollback and `undeployedChanges` describe what actually ran. **Status:** done — shipped `608f76011` and verified live on production 2026-10-19 (t005); m121 extends it to the build

## Tasks (in order)

| id   | title                                                                                  | est | depends_on         |
| ---- | -------------------------------------------------------------------------------------- | --- | ------------------ |
| t001 | Design: capture a release's own settings at adoption and use them through its rollout | 45m | — — **DONE** |
| t002 | Operator: record a release's settings and configuration snapshot when it is adopted   | 60m | w8/m120/t001 — **DONE** |
| t003 | Operator: render and record a held release from its own settings                      | 90m | w8/m120/t002 — **DONE** |
| t004 | Restart, Rollback and undeployedChanges read the corrected records; m119's bullet     | 45m | w8/m120/t003 — **DONE** |
| t005 | Render parity: Restart, Rollback, current configuration across REST, GraphQL, MCP, UI | 30m | w8/m120/t004 — **DONE** |
| t006 | Simplify the changed code                                                              | 20m | w8/m120/t005 — **DONE** |
| t007 | Test coverage: held release settings, records, Restart and Rollback after an overlap   | 50m | w8/m120/t005 — **DONE** |
| t008 | Close out the milestone and w8/m119                                                    | 10m | w8/m120/t006, t007 — **DONE** |

Eight tasks, 350m.

## Definition of done

Live on production after deploy, with the pinned CLI and an owned fixture deleted by ID with a 404 absence proof:

- **Settings during rollout.** A Docker web service gets two back-to-back saves: A, then B, whose start commands and one env var differ. A's pods run A's start command and A's env value until B rolls out. Its release record for A's generation holds A's start command and env, never B's.
- **Rollback.** Rolling back to A restores A's start command and env, both read back from the running pods.
- **Restart.** With G serving and H building, a Restart keeps serving G, H's row names the restart, and `undeployedChanges` reads `true` until a standard deploy applies H. This is w8/m119's open bullet; close m119 with it.
- **Controls.** A single deploy, a cancel of the pending deploy (w8/m117), and a suspend that ends a rollout (w5/m114) keep their behavior.
- **Tests.** Operator tests assert that the held release's applied template and record carry its own settings. The m119 envtests and the backend overlap tests stay green.

## Source + Goal linkage

- **Source:** the w8/m119 live DoD on 2026-10-19 (see [m119 t009](../blocked/m119/t009.md)). After two back-to-back saves, the release records of the held releases carried the newer saved settings: generation 2 (A) recorded B's start command, 4 (G) recorded H's, and 7 (X) recorded Z's. The operator projects a held release's Deployment and record from the current spec (`applyServingDeployment` ← `app.Spec`; `recordReleasePodTemplate` → `selectedRuntimeApp(app)`). Native builds bake the start command into the image, so A's pods still ran A. The Restart that re-released G "as recorded" found nothing undeployed, and a Rollback to A would restore B's settings. A Docker build would run A's image with B's command. This predates m119: the old code also applied the held release's template from the newer spec when its build resolved.
- **Goal linkage:** deploys, Rollback and Restart are core hosting primitives (ADR008, ADR004). ADR018 claims Render's Wait policy for overlapping deploys and Render's running-configuration contract for Restart (w5/m107). This milestone makes both hold after an overlap.
- **Expected outcome:**
  - after any overlap, what a release's history entry, Rollback and Restart say it ran is what it ran;
  - the dashboard and API show undeployed changes truthfully;
  - an overlapped Docker service never runs one release's image with another's command.
- **Why now:** it blocks w8/m119's closeout. Any two saves in a row reproduce it, and both Rollback and Restart act on the wrong record without telling the user.
- **Render parity included** because Restart, Rollback and `undeployedChanges` are tenant-visible on REST, GraphQL, MCP and the dashboard.
