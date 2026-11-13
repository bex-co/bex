# Render cron job dashboard: Builds and Runs pages

Captured 2026-11-13 for w2/m360. No authenticated Render account was available to this run (the repository configures no Render dashboard credential), so this records what Render publishes; the rows marked _unverified_ are not visible in it.

## What Render documents

- **Changelog, 2026-09-03** ("See your service's deploys on a new page"): "Services now have a new **Deploys** page … Cron jobs also have new **Builds** and **Runs** pages that separate build and run history."
- **[render.com/docs/cronjobs](https://render.com/docs/cronjobs)** (read 2026-11-13):
  - "After creating your cron job, you can track build progress and history on the **Builds** page."
  - "Your cron job's **Runs** page shows run history and logs for each run."
  - "To trigger a run manually, go to its **Runs** page in the Render Dashboard and click **Trigger Run**." A manual trigger during an active run "first _cancels_ the active run".

## How bex maps it

| Render | bex (w2/m360) |
| --- | --- |
| Cron sidebar: **Builds**, **Runs** in place of Deploys | `service-nav.tsx`: a cron job's top-level entries are Builds, Runs, Settings; other types keep Deploys |
| Builds page: build progress and history | `/services/$id/builds`: the deploy history list (a cron job's deploy is its build), titled Builds; rows open the existing deploy detail |
| Runs page: run history, logs for each run, Trigger Run | `/services/$id/runs`: the run list with status, start, duration and cancel; Trigger Run in the card header; each run's detail links **View logs**, the Logs tab over the run's window |
| Deep links `/cron/<srv-id>/builds`, `/cron/<srv-id>/runs` | The w5/m39 alias maps `/cron/<id>/<rest>` to `/services/<id>/<rest>` verbatim, so both land on the new routes |
| Old bex URLs | A cron job's `/services/$id/deploys` redirects to `/builds` with its filters; another type's `/builds` or `/runs` lands on its primary tab |

## Unverified

- The order of Builds and Runs in Render's sidebar, and whether Render keeps an Events entry for cron jobs (bex keeps Events under Monitor).
- The Runs page's columns, and whether a run's logs open in place or on the Logs page.
- Whether Render's Builds page differs from a web service's Deploys page beyond its name.

A future walk with a Render account should check these and file any drift against w2/m360's follow-ups.
