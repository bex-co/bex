# Render artifact — a service's Compute page and "compute plan" wording

**Captured:** 2026-11-19 from Render's public docs, read as Markdown (`render.com/docs/<page>.md`) plus the screenshots they embed (w6/m483 t001). No Render login is in `.env`, so Render's live dashboard was not captured. Primary sources:

- [New compute plans, and new IDs for existing plans](https://render.com/changelog/new-compute-plans-and-new-ids-for-existing-plans) (changelog, 2026-08-26)
- [Render Compute Plans](https://render.com/docs/compute-plans) and [Updates to Render Compute Plans](https://render.com/docs/compute-plans-update)
- [Scaling Render Services](https://render.com/docs/scaling)
- [Webhooks § Event types](https://render.com/docs/webhooks)
- [Render Postgres § Changing your compute plan](https://render.com/docs/postgresql-creating-connecting) and [Key Value § Changing compute plans](https://render.com/docs/key-value)

## What Render's docs say

**Wording.** The changelog: "'Compute plan' replaces 'instance type' across the dashboard, API, CLI, and docs." The plans update page: "Render introduced a new naming convention for compute plans (previously called 'instance types')". Its glossary: "Compute plans were previously known as _instance types_." Render's docs no longer call a plan an instance type anywhere except in the two names below.

**Plan IDs.** Each plan has a spec-based ID, `{CPU_COUNT}c-{RAM_GB}g` (`1c-2g`), with `mb` below 1 GB (`0.5c-512mb`); `free` keeps its name. The changelog: "you will see the IDs for plans change in the Render Dashboard" and "You can see the mapping of old plan names to new plan IDs in the Render Dashboard." The docs' plan tables put the **Plan ID** column first and the **Legacy Plan Name** beside it:

| Legacy Plan Name | Plan ID      |
| ---------------- | ------------ |
| (none)           | `free`       |
| Starter          | `0.5c-512mb` |
| Standard         | `1c-2g`      |
| Pro              | `2c-4g`      |
| Pro Plus         | `4c-8g`      |
| Pro Max          | `4c-16g`     |
| Pro Ultra        | `8c-32g`     |

bex's six paid compute rungs are exactly these sizes; `lego/types/tiers/aliases.go` (`computeInputAliases`, w1/m170) already holds the mapping, and `tiers.ComputeSpecID` reads it in reverse.

**The Compute page** (render.com/docs/compute-plans § Changing a service's compute plan, Dashboard tab):

1. "Open your service's **Compute** page in the Render Dashboard."
2. "Under **Compute**, click **Edit**:" (the screenshot under this step still shows the old Settings "Instance Type" row with an Update button)
3. "Select a new compute plan and click **Save**."
4. "Render automatically triggers a deploy to apply the new compute plan."

render.com/docs/scaling:

- Manual scaling: "open your service's **Compute** page and scroll down to the **Manual Scaling** section." "Drag the slider to the desired number of instances, or enter a value in the text box." "Click **Save Changes**."
- Autoscaling: "Enable autoscaling for your service from its **Compute** page". "Use the slider to set your desired minimum and maximum instance count", "Scroll down to set your target CPU and/or memory utilization", "Click **Save Changes**."
- "Each instance of a scaled service uses the same compute plan and is billed accordingly." Cron jobs are not scalable ("You can run multiple instances of a web service, private service, or background worker").

**Datastores.** Postgres: "From your database's **Plan** page … scroll down to the **Compute** section and click **Edit**", "Select a new compute plan from the list", "Click **Save**". Key Value: "open your instance's **Info** page and scroll down to the **Key Value Instance** section", "Under **Compute Plan**, click **Update**", "Select a new compute plan and click **Save Changes**". Both pages call the plan a "compute plan" throughout.

**Names Render still publishes with "instance type".**

- The `plan_changed` event: "A service's compute plan changed. In the Render Dashboard only, this event is referred to as _Instance Type Changed_." (render.com/docs/webhooks)
- Postgres's pre-2024 plans, which the Postgres docs still call "legacy instance types" (`/docs/postgresql-legacy-instance-types`). bex has no such plans.

## What the docs cannot settle, and the reading bex implements

| Open point | bex's reading (implemented under ADR008 § Render parity first) |
| --- | --- |
| Section order on the Compute page | **Compute** first (step 2 opens it at the top; Manual Scaling is "scroll down"). For the two scaling sections, the only layout evidence is Render's live Scaling page (2026-07-16, [manual-scaling.md](manual-scaling.md)): Autoscaling, then Manual Scaling, then Recent Metrics. bex keeps that order under Compute. |
| How the Compute section shows the current plan | A row with the plan ID, its legacy name, CPU and RAM, and **Edit**. Edit opens the plan cards in place with Cancel and **Save**; Save confirms and changes the plan. |
| Whether plan cards show IDs, names or both | Both: the plan ID first, the legacy name beside it, as the docs' tables and "the mapping of old plan names to new plan IDs in the Render Dashboard" describe. `free` has no legacy name; its card keeps the name Free. |
| URL and anchor | `/services/{id}/compute`. The old Plan URL redirects there; the old Scaling URL redirects to `#manual-scaling`. |
| Sidebar | One **Compute** entry where Scaling and Plan were (Render's Manage group carried Scaling; bex's Plan entry was bex-only). Cron jobs get Compute too: they have a compute plan but no scaling sections. |
| Settings page | Render's docs no longer send anyone to Settings to change a plan. bex keeps its Settings row as a summary with an Update link to Compute; a live capture may show Render dropped it. |
| Event name | The `plan_changed` event keeps the label "Instance type changed" in the event feed and metrics markers, as Render's dashboard keeps "Instance Type Changed". Its detail line names the plans ("Compute plan Starter → Standard"). |
| Datastore labels | "Compute plan" replaces "Instance type" on the Postgres and Key Value pages. Their layout is out of scope for w6/m483. |
