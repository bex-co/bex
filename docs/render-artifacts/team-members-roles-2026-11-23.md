# Render workspace roles — the role descriptions and role permissions table, 2026-11-23

Read on 2026-11-23 from [render.com/docs/team-members](https://render.com/docs/team-members) (its Markdown twin, `https://render.com/docs/team-members.md`), for w9/m619 and its siblings m620–m622. No sign-in is needed. The text below is Render's, reformatted only where its Markdown link syntax would not render here (`[x](<url>)` is written `[x](url)`).

## Role descriptions (verbatim)

- **Admin** (all plans)
  - Has full access to the workspace's resources _and_ organizational settings (such as member management, billing management, and secure login enforcement).
  - Can also designate individual project environments as protected.
  - A workspace's creator is automatically assigned this role.
- **Developer** (all plans)
  - Has access to the workspace's resources (services, environment groups, and so on), but _not_ organizational settings.
  - Access is limited for resources in a protected environment.
- **Contributor** (_Scale_ or higher). Similar to the _Developer_ role, with the following additional restrictions:
  - Can't view sensitive fields (billing info, connection strings, environment variables, and so on).
  - Can't access running services via SSH or the Shell tab in the Render Dashboard.
  - Can't create, modify, or delete most resources.
- **Viewer** (_Scale_ or higher)
  - Has _read-only_ access to most of the workspace's resources (services, environment groups, and so on).
  - Can't view service logs or sensitive fields (billing info, connection strings, environment variables, and so on).
- **Billing** (_Scale_ or higher)
  - Has full access to the workspace's billing settings, including updating payment method. Can also view non-sensitive details of the workspace's resources (such as members and service names).

## Role permissions (verbatim)

> 🟢 Permitted
>
> ❌ Not permitted
>
> 🟨 Permitted with restrictions (details vary by permission)

### Workspace administration

| Permission | Admin | Developer | Contributor | Viewer | Billing |
| --- | --- | --- | --- | --- | --- |
| _View workspace members_ | 🟢 | 🟢 | 🟢 | 🟢 | 🟢 |
| _Edit workspace settings_ | 🟢 | ❌ | ❌ | ❌ | 🟨 Billing settings only. |
| _Add/remove workspace members_ | 🟢 | ❌ | ❌ | ❌ | ❌ |
| _Export [audit logs](https://render.com/docs/audit-logs.md)_ | 🟢 | ❌ | ❌ | ❌ | ❌ |
| _View billing details_ | 🟢 | 🟢 | ❌ | ❌ | 🟢 |
| _Edit payment method_ | 🟢 | ❌ | ❌ | ❌ | 🟢 |
| _Leave workspace_ | 🟢 | 🟢 | 🟢 | 🟢 | 🟢 |
| _Delete workspace_ | 🟢 | ❌ | ❌ | ❌ | ❌ |

### Projects and environments

| Permission | Admin | Developer | Contributor | Viewer | Billing |
| --- | --- | --- | --- | --- | --- |
| _View projects_ | 🟢 | 🟢 | 🟢 | 🟢 | 🟢 |
| _Create/modify projects_ | 🟢 | 🟢 | ❌ | ❌ | ❌ |
| _Delete projects_ | 🟢 | 🟨 Can't delete a project with at least one [protected environment](https://render.com/docs/projects.md#protected-environments). | ❌ | ❌ | ❌ |
| _View environments_ | 🟢 | 🟢 | 🟢 | 🟢 | 🟢 |
| _Create environments_ | 🟢 | 🟢 | ❌ | ❌ | ❌ |
| _Modify/delete environments_ | 🟢 | 🟨 Non-[protected environments](https://render.com/docs/projects.md#protected-environments) only. | ❌ | ❌ | ❌ |
| _Designate an environment as [protected](https://render.com/docs/projects.md#protected-environments) or [network-isolated](https://render.com/docs/projects.md#blocking-cross-environment-traffic)_ | 🟢 | ❌ | ❌ | ❌ | ❌ |
| _Move resources into or out of an environment_ | 🟢 | 🟨 Non-[protected environments](https://render.com/docs/projects.md#protected-environments) only. | ❌ | ❌ | ❌ |
| _Manage environment secrets_ | 🟢 | 🟨 Non-[protected environments](https://render.com/docs/projects.md#protected-environments) only. | ❌ | ❌ | ❌ |

### Services and datastores

| Permission | Admin | Developer | Contributor | Viewer | Billing |
| --- | --- | --- | --- | --- | --- |
| _View services_ | 🟢 | 🟢 | 🟢 | 🟢 | 🟢 |
| _Create services_ | 🟢 | 🟨 Non-[protected environments](https://render.com/docs/projects.md#protected-environments) only. | ❌ | ❌ | ❌ |
| _Modify service configuration_ | 🟢 | 🟨 Can't perform potentially destructive modifications to services in a [protected environment](https://render.com/docs/projects.md#protected-environments). | ❌ | ❌ | ❌ |
| _Trigger service deploys_ | 🟢 | 🟢 | 🟢 | ❌ | ❌ |
| _Trigger [rollbacks](https://render.com/docs/rollbacks.md)_ | 🟢 | 🟢 | 🟢 | ❌ | ❌ |
| _Delete services_ | 🟢 | 🟨 Non-[protected environments](https://render.com/docs/projects.md#protected-environments) only. | ❌ | ❌ | ❌ |
| _View connection strings and credentials for datastores_ | 🟢 | 🟨 Non-[protected environments](https://render.com/docs/projects.md#protected-environments) only. | ❌ | ❌ | ❌ |
| _Manage [inbound IP rules](https://render.com/docs/inbound-ip-rules.md)_ | 🟢 | 🟨 Varies by resource type. [See details](https://render.com/docs/inbound-ip-rules.md#permissions). | ❌ | ❌ | ❌ |
| _Access running services via SSH or the Shell tab_ | 🟢 | 🟨 Non-[protected environments](https://render.com/docs/projects.md#protected-environments) only. | ❌ | ❌ | ❌ |

### Observability

| Permission | Admin | Developer | Contributor | Viewer | Billing |
| --- | --- | --- | --- | --- | --- |
| _View service events_ | 🟢 | 🟢 | 🟢 | 🟢 | ❌ |
| _View [service metrics](https://render.com/docs/service-metrics.md)_ | 🟢 | 🟢 | 🟢 | 🟢 | ❌ |
| _View [service logs](https://render.com/docs/logging.md)_ | 🟢 | 🟢 | 🟢 | ❌ | ❌ |

### Integrations

| Permission | Admin | Developer | Contributor | Viewer | Billing |
| --- | --- | --- | --- | --- | --- |
| _Configure [notification settings](https://render.com/docs/notifications.md)_ | 🟢 | ❌ | ❌ | ❌ | ❌ |
| _Create and configure [webhooks](https://render.com/docs/webhooks.md)_ | 🟢 | ❌ | ❌ | ❌ | ❌ |

## How bex reads it (w9/m619)

- **Settings are the Developer's.** "Modify service configuration" and "Manage inbound IP rules" are ❌ for a Contributor, and the description says a Contributor "can't create, modify, or delete most resources". bex puts every service and datastore setting on `can_create` (Developer and up): a service's name, plan, instance count, autoscaling, idle timeout, shutdown delay, health check path, build filter, auto-deploy, cron schedule, maintenance mode, edge cache profile, subdomain policy, inbound IP rules, custom domains and notification override, and a Postgres or Key Value instance's settings, plan and version.
- **Runtime actions stay the Contributor's.** The table restricts none of them, and the Contributor is "similar to the Developer role" apart from the listed restrictions: restart, suspend, resume, deploy and deploy cancel, cron run and cancel, edge cache purge, one-off job cancel, and Postgres restart, failover and export. This is bex's reading, not a row of the table.
- **Not decided here.** "Trigger rollbacks" (w9/m622), "View billing details" (w9/m620) and the Billing role's observability rows (w9/m621) are their own milestones.

## How bex reads it (w9/m620)

- **Billing details are read by Admin, Developer and Billing.** "View billing details" is 🟢 for those three and ❌ for Contributor and Viewer, whose descriptions both say they "can't view sensitive fields (billing info, …)". bex's `can_view_billing` (`developer or admin or billing`) gates the usage with its estimated and real cost, invoices and credit grants, the billing readiness and the pipeline spend limit.
- **Editing stays Admin and Billing.** "Edit payment method" is 🟢 for Admin and Billing only, and the Billing role "has full access to the workspace's billing settings". `can_manage_billing` keeps Checkout, the Customer Portal and the pipeline spend limit.

## How bex reads it (w9/m621)

- **Events and metrics are every role's but Billing's.** "View service events" and "View service metrics" are 🟢 for Admin, Developer, Contributor and Viewer and ❌ for Billing, whose description grants it only "non-sensitive details of the workspace's resources (such as members and service names)". bex's `can_view_activity` (`viewer or contributor or developer or admin`) gates a service's events, every service, datastore and project metric, and the deploy and cron run history: a deploy's and a cron run's start and end are service events on Render (`deploy_started`, `deploy_ended`, `cron_job_run_started`, `cron_job_run_ended`).
- **Names and details stay every role's.** The service, datastore and project lists and details stay `can_view`, Billing included.
- **Logs stay Contributor and up.** "View service logs" is ❌ for Viewer and Billing; `can_view_logs` is unchanged.

## How bex reads it (w9/m622)

- **Every deploy and rollback is the Contributor's.** "Trigger service deploys" and "Trigger rollbacks" are 🟢 for Admin, Developer and Contributor and ❌ for Viewer and Billing. Render's Manual Deploy menu offers "Deploy a specific commit" and "Clear build cache & deploy" as deploys ([render.com/docs/deploys](https://render.com/docs/deploys), read 2026-11-24), and its API's deploy takes `commitId`, `imageUrl` and `clearCache`. bex puts every deploy, with any of those (and `buildId`), and every rollback on `can_operate`.
- **The source stays a setting.** "Modify service configuration" is ❌ for a Contributor, so what a deploy may name stays bounded by the saved source: the configured repository, another tag or digest of the configured image (Render's same-image rule) and the linked build source, each `can_create` to change.
- **Still open.** Reading a chosen-commit or chosen-image deploy as part of "Trigger service deploys" is bex's recommendation; the user may sign off keeping those Developer-only as an ADR018 divergence ([ADR024](../ADR024-members.md#the-contributor-boundary-deploys-and-runtime-actions-not-settings-w1m68-w9m619-w9m622)).
