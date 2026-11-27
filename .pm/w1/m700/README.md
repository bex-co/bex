# w1 · m700 — The Environment page's save button offers Render's choices for each kind of service

**Worker:** worker1 **Goal:** every kind of service, the build source and the environment group save their variables with Render's choices, in Render's order and with Render's default. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](t001.md) | Record Render's save choices per kind in `docs/render-artifacts/service-environment-page.md` | 30m | — |
| [t002](t002.md) | Each kind of service offers Render's save choices | 1h30m | t001 |
| [t003](t003.md) | The main button follows auto-deploy, as Render's does | 1h | t002 |
| [t004](t004.md) | A build source saves with "Save and rebuild" or "Save only" | 2h | t001 |
| [t005](t005.md) | A suspended service shows one plain "Save" | 20m | t002 |
| [t006](t006.md) | Locales, and a walk of each kind on dev-1 and production | 1h | t003, t004, t005 |
| [t007](t007.md) | Render parity | 20m | t006 |
| [t008](t008.md) | Simplify | 15m | t007 |
| [t009](t009.md) | Test coverage | 30m | t007 |
| [t010](t010.md) | Closeout | 10m | t008, t009 |

Total: **10 tasks, about 7h35m**.

## Definition of done

1. A web service, private service or background worker built from a repository offers "Save, rebuild, and deploy", "Save and deploy" and "Save only". One that is image-backed, or built from a build source, offers "Save and deploy" and "Save only".
2. A static site offers "Save and deploy" (which rebuilds the site, as it does today) and "Save only". A cron job offers "Save, rebuild, and apply on next run" (only when built from a repository), "Save and apply on next run" and "Save only".
3. A build source's Environment tab offers "Save and rebuild" and "Save only". "Save and rebuild" saves and starts one build of the build source, through REST, GraphQL and MCP as well as the dashboard.
4. The main button runs the first choice when the service has auto-deploy on, and "Save only" when it is off. A build source's main button is "Save only". An environment group's main button is its first choice when any linked service that is not suspended has auto-deploy on, and "Save only" otherwise.
5. A suspended service shows one plain "Save" button with no menu.
6. All of the above walked on production at 1440 and 390 px, in all eight locales.

## Source + Goal linkage

- **Source:** `/pm-brainstorm one aspect of render parity to w1` (2026-11-26), item 1 (a service's Environment page), verified before filing.
  - Render's public dashboard bundle was read 2026-11-26 without sign-in (entry `index-D5V4ZgbE.js`, 459 chunks). Chunk hashes change with every Render deploy, so find the code by the names and strings quoted here.
  - In the `VariablesActionButtons` chunk, `xt({service, allowDeployOnSave})` builds the choices:
    - A suspended service (`state==="Suspended"`) gets none, so the split button is one plain "Save".
    - An `ArtifactSource` (Render's Build Source) gets "Save and rebuild" / "Save only".
    - A static site gets "Save and deploy" / "Save only".
    - A cron job gets "Save, rebuild, and apply on next run" when buildable, then "Save and apply on next run" / "Save only".
    - Every other service gets "Save, rebuild, and deploy" when buildable, then "Save and deploy" / "Save only".
    - Buildable means not image-backed (`env.id!=="image"`) and no `artifactSourceID`.
  - `St` picks the default: `autoDeploy ? options[0].id : "no-deploy"`. The service page passes `autoDeploy: t.autoDeploy===true`. A build source's fragment has no `autoDeploy`, so it defaults to "Save only". `EditableEnvGroup` offers all three choices and defaults to the first when any linked, non-suspended service has auto-deploy on.
  - A workflow's "Save, rebuild, and register" is left out: `.pm/DO_NOT_DO.md` rules workflows out.
  - No flag gates this path. Render's docs list only the three generic choices, which does not contradict the bundle.
- **bex today (checked 2026-11-26):**
  - `dashboard/src/features/services/components/service-environment-editor.tsx` has one `SaveChoice` (`:125`) and one menu (`:1192-1253`) for every kind. The main button is "Save and deploy" unless the service is suspended (w4/m234). The `dockerBuild` prop (`:160-171`) only changes the secret-file dialog's text.
  - On an image-backed service, "Save, rebuild, and deploy" calls `deploys.Service.Rebuild` (`lego/backend/internal/deploys/service.go:723`), which pulls the saved image again and builds nothing. On a service built from a build source it redeploys the build source's build.
  - A static site's "Save and deploy" already rebuilds (`ServedArtifact`, `lego/backend/internal/rollout/served_image.go:128-133`), so its two deploying choices do the same thing.
  - A cron job already applies new values on its next run. Only the labels differ.
  - The build source's tab saves only (`dashboard/src/features/build-sources/components/build-source-environment-tab.tsx:110-118`). The backend refuses every other mode (`lego/backend/internal/secrets/batch.go:114-117`), and its comment says that matches Render, which the bundle contradicts.
  - The editor already has the service's `autoDeploy`, `imagePath` and `buildSourceId` from `useServer`.
- **Not a duplicate of:**
  - w5/m44, which built the editor and its three choices.
  - w4/m234 (saves on a suspended service) and w4/m180 (what a configuration-only deploy reruns).
  - ADR018's env-vars row ([environment-config.md](../../../docs/ADR018/environment-config.md)), which claims "the three Render save choices" without Render's rule per kind.
  - [w10/m375](../../w10/blocked/m375/README.md) (auto-deploy reads what will actually happen) is adjacent: this milestone's default reads the same `autoDeploy`.
- **Goal linkage:** Render parity first ([ADR008](../../../docs/ADR008-vision.md#render-parity-first)).
- **Expected outcome:** the save button never offers a choice that does not apply to the service, and a service whose owner turned auto-deploy off saves without deploying unless the owner picks a deploy, as on Render.
- **Why now:** bex's main button deploys a service whose owner turned auto-deploy off, where Render's only saves. That changes what one of the dashboard's most-used buttons does, not only its label. The rebuild on an image-backed service and the two identical static-site choices mislead today.
- **Render parity task:** included (t007). The dashboard's save button changes, and the build source's save gains a rebuild mode on REST, GraphQL and MCP.
