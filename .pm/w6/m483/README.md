# w6 · m483 — A service's Compute page holds its plan, manual scaling and autoscaling, with Render's "compute plan" wording and plan IDs

**Worker:** worker6 **Goal:** as on Render since 2026-08-26, a service's dashboard has one **Compute** page with its compute plan, Manual Scaling and Autoscaling; plans are called "compute plans", not "instance types", and plan cards show Render's spec-based plan IDs beside the legacy names. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [Record Render's Compute page and wording](t001.md) | 30m | — |
| t002 | [Merge the Plan and Scaling tabs into a Compute page; the old paths redirect](t002.md) | 1h30m | t001 |
| t003 | [The Compute page's pending skeleton matches the ready page at 1440 and 390 px](t003.md) | 45m | t002 |
| t004 | [Plan cards show Render's plan ID beside the legacy name](t004.md) | 45m | t001 |
| t005 | ["Compute plan" replaces "instance type" in the dashboard and bex-api's messages](t005.md) | 1h | t001 |
| t006 | [Links that opened Plan or Scaling open Compute](t006.md) | 30m | t002 |
| t007 | [Render parity](t007.md) | 20m | t003, t004, t005, t006 |
| t008 | [Simplify](t008.md) | 15m | t007 |
| t009 | [Test coverage](t009.md) | 30m | t007 |
| t010 | [Closeout](t010.md) | 10m | t008, t009 |

## Definition of done

- `/services/{id}/compute` shows the service's compute plan (with Edit), Manual Scaling and Autoscaling in Render's order; `/services/{id}/plan` and `/services/{id}/scaling` redirect to it (scaling to its Manual Scaling section); the service nav reads "Compute".
- Every plan card and the current-plan row show Render's plan ID (e.g. `0.5c-512mb` for Starter) beside the legacy name, from the same alias table Blueprints use (`lego/types/tiers/aliases.go`).
- No English dashboard string or bex-api error calls a plan an "instance type", except names Render itself still publishes, listed in t001's record (e.g. the "Instance Type Changed" event if Render keeps it).
- The Compute page's pending skeleton matches the ready page at 1440 and 390 px.
- `cd dashboard && yarn lint && yarn test` and `cd lego/backend && go test ./...` pass.
- Checked on production on a paid fixture service.

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w6` 2026-11-17 (item 2 after verification), approved by the user's `/pmx them all to w6` 2026-11-18. Found by the Render changelog sweep of 2026-08-15 to 2026-11-17.
  - Render changelog 2026-08-26, [New compute plans, and new IDs for existing plans](https://render.com/changelog/new-compute-plans-and-new-ids-for-existing-plans): "'Compute plan' replaces 'instance type' across the dashboard, API, CLI, and docs"; "you will see the IDs for plans change in the Render Dashboard"; "You can see the mapping of old plan names to new plan IDs in the Render Dashboard."
  - [render.com/docs/compute-plans](https://render.com/docs/compute-plans): "Open your service's **Compute** page in the Render Dashboard. Under **Compute**, click **Edit** … Select a new compute plan and click **Save**." [render.com/docs/scaling](https://render.com/docs/scaling): "open your service's Compute page and scroll down to the Manual Scaling section"; "Enable autoscaling for your service from its Compute page".
  - bex: two tabs, `dashboard/src/routes/services.$serviceId.plan.tsx` and `services.$serviceId.scaling.tsx` (w7/m43 matched Render's page before the change); about 22 English strings say "Instance type"; bex-api errors say it (`lego/backend/internal/core/quota.go:138,142`, `apps/disks.go:310`, `deploys/service.go:1352`); plan cards show the legacy names only. ADR018's plan-ID row (Blueprint compute plan IDs, w1/m170) covers Blueprints and the API, not the dashboard.
- **Goal linkage:** ADR008 § Render parity first; bex is AI-native, and agents follow Render's docs, which now point to a Compute page and plan IDs.
- **Expected outcome:** a user or agent following render.com/docs/compute-plans or /docs/scaling finds the same page, section names, wording and plan IDs on bex.
- **Why now:** the change is three months old and every service page shows the old layout; the plan-ID half for Blueprints and larger plans is already owned (w1/m170, `w6/blocked/114`, `w6/blocked/m332`), the dashboard half is not.
- **Open point (not a user decision):** Render's live dashboard was not captured (no Render login in `.env`); the layout follows Render's docs. t001 records what the docs cannot settle, and the tasks implement the docs' reading by default under ADR008.
- **Render parity task included:** dashboard UI and bex-api error text change.
