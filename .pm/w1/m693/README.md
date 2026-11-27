# w1 · m693 — A Free service's Shell and Disk tabs open Render's upgrade dialog

**Worker:** worker1 **Goal:** on a Free web service, private service or background worker, the Shell and Disk tabs open Render's "Enable Shell Access" / "Enable Disk Access" dialog. The Shell shows its sample terminal behind it. The dialog lists only what bex's Starter plan really adds. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](t001.md) | Record Render's dialog, and check each "also includes" line against bex's Starter plan | 30m | — |
| [t002](t002.md) | The shared upgrade dialog: Cancel goes to Events, Review upgrade options goes to Compute | 1h15m | t001 |
| [t003](t003.md) | The Shell tab on Free: Render's sample terminal behind the dialog | 45m | t002 |
| [t004](t004.md) | The Disk tab on Free: the dialog in place of the tab's contents | 30m | t002 |
| [t005](t005.md) | Eight locales, and the Shell and Disk loading frames for a Free service | 45m | t003, t004 |
| [t006](t006.md) | Walk a Free service's Shell and Disk tabs on dev-1 and production | 45m | t005 |
| [t007](t007.md) | Render parity | 20m | t006 |
| [t008](t008.md) | Simplify | 15m | t007 |
| [t009](t009.md) | Test coverage | 30m | t007 |
| [t010](t010.md) | Closeout | 10m | t008, t009 |

Total: **10 tasks, about 5h45m**.

## Definition of done

1. Opening Shell or Disk on a Free service opens a dialog titled "Enable Shell Access" or "Enable Disk Access", with Render's description and Render's sentence ("Shell is not supported for free compute plans." / "Disks are not supported for free compute plans.").
2. The dialog lists what upgrading to Starter "also includes", leaving out any line bex's Starter does not add. One-off jobs is left out (see the divergence below).
3. Cancel or close goes to the service's Events, and **Review upgrade options** goes to its Compute page.
4. Behind the Shell's dialog sits Render's sample terminal, ending at the service's `{id}:/#` prompt.
5. A paid service's tabs are unchanged, and so is a suspended paid service's. The loading frames match the Free ready state.
6. Walked on production at 1440 and 390 px in all eight locales.

## Source + Goal linkage

- **Source:** `/pm-brainstorm one aspect of render parity to w1` (2026-11-26), item 3, verified before filing.
  - Render's public dashboard bundle was read 2026-11-26 without sign-in (entry `index-D5V4ZgbE.js`). Find the code by the strings quoted here.
  - `index` defines the dialog (`unsupportedMessage`, `feature`, `referringTab`): title `Enable ${feature}`, description "Upgrade your instance and get more out of Render.".
  - Its body is the unsupported sentence, then "Upgrading to the **Starter** compute plan also includes:" over ✓ Zero downtime, Web shell and SSH access, Persistent Disks, One-off jobs, Scaling.
  - Its footer: Cancel goes to `${servicePath}/events`, and **Review upgrade options** goes to `/compute` (`/settings#compute` for a cron job).
  - The Disk tab renders only the dialog when the plan is `Free` (`freeTierPlans`).
  - `ServiceShellStarter` checks the shell permission first, then Free (sample terminal plus dialog), then running, else "The shell is unavailable for suspended services.". The sample terminal reads "==> Welcome to Render Shell.", "Here are some things you can do with the shell:", " - Remotely interact with your service", " - ...", "🧑‍🚀 Be careful, astronaut! You are now in a live production environment." and `{id}:/#`.
  - No flag gates it. The analytics events it fires (`Upsell Upgrade Modal Opened`) are not part of this work.
- **bex today (checked 2026-11-26):**
  - The Shell tab shows a "Shell access isn't available" card with "SSH needs a paid compute plan. Free services don't include shell access." and a Change compute plan link (`dashboard/src/features/services/components/service-shell-page.tsx`, `lib/ssh-remedy.ts`, w4/143).
  - The Disk tab shows `services.diskPaidOnly` (`components/disk-section.tsx:118-120`).
  - bex's Free plan is limited to one instance (`lego/backend/internal/apps/service.go:5191`), and refuses autoscaling, disks and SSH. Free web services sleep.
- **Divergence for the user's sign-off:** the list omits "One-off jobs", because one-off jobs are an anti-goal ([`.pm/DO_NOT_DO.md`](../../DO_NOT_DO.md), "One-off jobs … and ephemeral shell instances"). Listing a feature bex will not ship would be false. t007 records it in ADR018. If the user wants it listed, that needs the anti-goal reversed first.
- **Not a duplicate of:**
  - w4/143, which shows the Free and suspended reasons and remedy links that this dialog replaces on Free.
  - w3/124, which fixes the Shell and Metrics loading frames. t005 coordinates with it.
- **Goal linkage:** Render parity first ([ADR008](../../../docs/ADR008-vision.md#render-parity-first)).
- **Expected outcome:** a Free user who opens Shell or Disk sees what Render shows: why the feature is unavailable, what Starter adds, and one click to the plan picker.
- **Why now:** every Free service's Shell and Disk tabs differ from Render's today, and this is independent of w1/m691–m692.
- **Render parity task:** included (t007). Dashboard pages change. REST, GraphQL and MCP do not.
