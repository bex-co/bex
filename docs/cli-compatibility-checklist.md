# Render CLI compatibility checklist

The full command surface of the official [Render CLI](https://render.com/docs/cli) (`render-oss/cli` **v2.21.0**), enumerated straight from `render --help` for bex compatibility tracking. Every command, subcommand, and flag is a dedicated checklist item so each can be graded independently.

Legend: `[x]` supported · `[~]` supported with a limitation · `[ ]` not yet supported / unverified · `[-]` deliberate non-goal.

Global flags on every command: `--confirm` (skip confirmation prompts), `-o, --output <interactive|json|yaml|text>` (auto-switches to `text` on non-TTY), `-h, --help`, `-v, --version`.

> Regenerate this tree with `render <subcommand> --help` (recurse into each `SUBCOMMANDS` block). Grouping below mirrors the CLI's own `render --help` sections.

## Core

- [ ] **`deploys`** — list, create, and cancel deploys
  - [ ] `deploys list <serviceID>` — list deploys for a service
  - [ ] `deploys create <serviceID>` — trigger a deploy and stream logs
    - [ ] `--clear-cache` — clear build cache before deploying
    - [ ] `--commit <id>` — deploy the specified commit ID
    - [ ] `--image <url>` — deploy the specified Docker image URL
    - [ ] `--wait` — wait for completion and exit non-zero on failure
  - [ ] `deploys cancel <serviceID> <deployID>` — cancel a running deploy
- [ ] **`jobs`** — create and manage one-off jobs
  - [ ] `jobs list <serviceID>` — list jobs for a service
  - [ ] `jobs create <serviceID>` — create a one-off job (uses the service image)
    - [ ] `--start-command <cmd>` — set the job start command
    - [ ] `--plan-id <id>` — set the plan ID for the job
  - [ ] `jobs cancel <serviceID> <jobID>` — cancel a running job
- [ ] **`keyvalues`** (alias `kv`) — manage Render Key Value instances
  - [ ] `keyvalues create` — create a Key Value instance
    - [ ] `--name <string>` — instance name (generated if unset)
    - [ ] `--plan <free|starter|standard|pro|pro_plus>` — plan
    - [ ] `--region <frankfurt|ohio|oregon|singapore|virginia>` — region
    - [ ] `--memory-policy <cache|queue|raw policy>` — eviction policy
    - [ ] `--ip-allow-list cidr=…,description=…` — inbound IP allow-list entry (repeatable)
    - [ ] `--workspace <id|name>` — target workspace
    - [ ] `--project <id|name>` — scope environment lookup to a project
    - [ ] `--environment <id|name>` — target environment
  - [ ] `keyvalues list` — list Key Value instances
  - [ ] `keyvalues get <id|name>` — get instance details
  - [ ] `keyvalues update <id|name>` — update an instance
    - [ ] `--name <string>` — rename the instance
    - [ ] `--plan <string>` — change plan
    - [ ] `--memory-policy <enum>` — change eviction policy
    - [ ] `--ip-allow-list cidr=…,description=…` — replace the allow-list (repeatable)
    - [ ] `--clear-ip-allow-list` — remove all allow-list entries
  - [ ] `keyvalues suspend <id|name>` — suspend an instance
  - [ ] `keyvalues resume <id|name>` — resume a suspended instance
  - [ ] `keyvalues delete <id|name>` — delete an instance
- [ ] **`logs`** — view logs for services and datastores (single command)
  - [ ] query mode — filter over a time range and print
  - [ ] `--tail` — stream new logs live
  - [ ] `-r, --resources <ids>` — filter by resource IDs (required in non-interactive mode)
  - [ ] `--instance <ids>` — filter by instance IDs
  - [ ] `--start <time>` — logs at or after this time
  - [ ] `--end <time>` — logs at or before this time
  - [ ] `--direction <backward|forward>` — query direction
  - [ ] `--limit <count>` — cap the number of logs (default 100)
  - [ ] `--text <query>` — filter by text values
  - [ ] `--level <levels>` — filter by log levels
  - [ ] `--type <types>` — filter by log types
  - [ ] `--host <hosts>` — filter by host values
  - [ ] `--status-code <codes>` — filter by HTTP status codes
  - [ ] `--method <methods>` — filter by HTTP methods
  - [ ] `--path <paths>` — filter by request paths
  - [ ] `--task-id <ids>` — filter by task IDs
  - [ ] `--task-run-id <ids>` — filter by task run IDs
- [ ] **`postgres`** (alias `pg`) — manage Render Postgres databases
  - [ ] `postgres create` — create a database
    - [ ] `--name <string>` — database name (generated if unset)
    - [ ] `--plan <free|basic_*|pro_*|accelerated_*>` — plan
    - [ ] `--region <frankfurt|ohio|oregon|singapore|virginia>` — region
    - [ ] `--version <int>` — Postgres major version (default 18)
    - [ ] `--disk-size-gb <int>` — disk size in GB
    - [ ] `--disk-autoscaling` — enable disk autoscaling
    - [ ] `--high-availability` — enable HA (Pro plans and above)
    - [ ] `--read-replica <name>` — create a read replica (repeatable)
    - [ ] `--ip-allow-list cidr=…,description=…` — inbound IP allow-list entry (repeatable)
    - [ ] `--database-name <string>` — initial database name
    - [ ] `--database-user <string>` — initial database user
    - [ ] `--datadog-api-key <string>` — Datadog monitoring key
    - [ ] `--datadog-site <string>` — Datadog region/site
    - [ ] `--workspace <id|name>` — target workspace
    - [ ] `--project <id|name>` — scope environment lookup to a project
    - [ ] `--environment <id|name>` — target environment
  - [ ] `postgres list` — list databases
  - [ ] `postgres get <id|name>` — get database details
  - [ ] `postgres update <id|name>` — update a database
    - [ ] `--name <string>` — rename the database
    - [ ] `--plan <string>` — change plan
    - [ ] `--disk-size-gb <int>` — change disk size
    - [ ] `--disk-autoscaling` — toggle disk autoscaling
    - [ ] `--high-availability` — toggle HA
    - [ ] `--ip-allow-list cidr=…,description=…` — replace the allow-list (repeatable)
    - [ ] `--clear-ip-allow-list` — remove all allow-list entries
    - [ ] `--datadog-api-key <string>` — set/clear Datadog key
    - [ ] `--datadog-site <string>` — set Datadog region/site
    - [ ] `--project <id|name>` — narrow lookup to a project
    - [ ] `--environment <id|name>` — narrow lookup to an environment
  - [ ] `postgres suspend <id|name>` — suspend a database
  - [ ] `postgres resume <id|name>` — resume a suspended database
  - [ ] `postgres delete <id|name>` — delete a database
- [ ] **`restart <resourceID>`** — restart a service by resource ID
- [ ] **`services`** — list services and datastores; bare `services` lists them
  - [ ] `-e, --environment-ids <ids>` — filter list by environment IDs
  - [ ] `--include-previews` — include preview environments in the list
  - [ ] `services create` — create a service (or clone with `--from`)
    - [ ] `--name <string>` — service name
    - [ ] `--type <string>` — service type
    - [ ] `--runtime <string>` — runtime environment
    - [ ] `--repo <url>` — Git repository URL
    - [ ] `--branch <string>` — Git branch
    - [ ] `--image <url>` — Docker image URL
    - [ ] `--plan <string>` — service plan
    - [ ] `--region <string>` — deployment region
    - [ ] `--num-instances <count>` — number of instances
    - [ ] `--build-command <cmd>` — build command
    - [ ] `--start-command <cmd>` — start command
    - [ ] `--pre-deploy-command <cmd>` — pre-deploy command
    - [ ] `--cron-command <cmd>` — cron command
    - [ ] `--cron-schedule <schedule>` — cron schedule
    - [ ] `--health-check-path <path>` — health check path
    - [ ] `--auto-deploy` — enable auto-deploy (default true)
    - [ ] `--previews <mode>` — preview generation mode
    - [ ] `--publish-directory <path>` — publish directory
    - [ ] `--root-directory <path>` — root directory
    - [ ] `--env-var KEY=VALUE` — set an env var (repeatable)
    - [ ] `--secret-file NAME:PATH` — set a secret file (repeatable)
    - [ ] `--registry-credential <cred>` — registry credential
    - [ ] `--ip-allow-list cidr=…,description=…` — inbound IP allow-list entry (repeatable)
    - [ ] `--build-filter-path <path>` — build filter path (repeatable)
    - [ ] `--build-filter-ignored-path <path>` — build filter ignored path (repeatable)
    - [ ] `--maintenance-mode` — enable maintenance mode
    - [ ] `--maintenance-mode-uri <uri>` — maintenance mode URI
    - [ ] `--max-shutdown-delay <seconds>` — max shutdown delay
    - [ ] `--environment-id <id>` — target environment
    - [ ] `--from <serviceID>` — clone config from an existing service
  - [ ] `services update <service>` — update a service's configuration
    - [ ] `--name <string>` — rename the service
    - [ ] `--plan <string>` — change plan
    - [ ] `--runtime <enum>` — runtime environment
    - [ ] `--repo <string>` — Git repository URL
    - [ ] `--branch <string>` — Git branch
    - [ ] `--image <string>` — Docker image URL
    - [ ] `--build-command <string>` — build command
    - [ ] `--start-command <string>` — start command
    - [ ] `--pre-deploy-command <string>` — pre-deploy command
    - [ ] `--cron-command <string>` — cron command
    - [ ] `--cron-schedule <string>` — cron schedule
    - [ ] `--health-check-path <string>` — health check path
    - [ ] `--auto-deploy` — toggle auto-deploy
    - [ ] `--previews <enum>` — preview generation mode
    - [ ] `--publish-directory <string>` — publish directory
    - [ ] `--root-directory <string>` — root directory
    - [ ] `--registry-credential <string>` — registry credential
    - [ ] `--ip-allow-list cidr=…,description=…` — replace allow-list (repeatable)
    - [ ] `--build-filter-path <path>` — build filter path (repeatable)
    - [ ] `--build-filter-ignored-path <path>` — build filter ignored path (repeatable)
    - [ ] `--maintenance-mode` — toggle maintenance mode
    - [ ] `--maintenance-mode-uri <string>` — maintenance mode URI
    - [ ] `--max-shutdown-delay <int>` — max shutdown delay
  - [ ] `services instances <serviceID>` — list instances for a service
  - [ ] `services delete <serviceID>` — delete a service
- [-] **`workflows`** — manage Render Workflows (deliberate bex non-goal)
  - [-] `workflows list` — list workflow services
  - [-] `workflows create` — create a workflow service
  - [-] `workflows init` — scaffold a new Workflows project
  - [-] `workflows dev` — start a workflow service in development mode
  - [-] `workflows start` — start a task run (shortcut for `tasks runs start`)
  - [-] `workflows cancel` — cancel a task run (shortcut for `tasks runs cancel`)
  - [-] `workflows tasks` — list tasks and manage their runs
    - [-] `workflows tasks list <versionID>` — list tasks in a workflow version
    - [-] `workflows tasks runs` — start, list, and inspect task runs
      - [-] `… runs start` — start a task run (`--task`, `--input`)
      - [-] `… runs list` — list runs for a task (`--task`)
      - [-] `… runs show <runID>` — show task-run details
      - [-] `… runs cancel <runID>` — cancel a running task run
  - [-] `workflows versions` — list and release workflow versions
    - [-] `workflows versions list <workflowID>` — list versions of a workflow
    - [-] `workflows versions release <workflowID>` — release a new version
- [ ] **`workspaces`** — list workspaces available to your account

## Auth

- [ ] **`login`** — log in via the Render Dashboard (browser/device flow)
- [ ] **`logout`** — log out
- [ ] **`whoami`** — display the current user
- [ ] **`workspace`** — manage the CLI's active workspace
  - [ ] `workspace current` — show the selected workspace
  - [ ] `workspace set <id>` — set the active workspace

## Session

- [ ] **`kv-cli [id|name]`** — open a redis-cli/valkey-cli session (interactive only; pass-through args after `--`)
- [ ] **`pgcli [id|name]`** — open a pgcli session (interactive only; pass-through args after `--`)
- [ ] **`psql [id|name]`** — open a psql session
  - [ ] `-c, --command <SQL>` — execute SQL non-interactively (pass-through psql args after `--`)
- [ ] **`ssh [serviceID|serviceName|instanceID]`** — SSH into a service instance (interactive only; pass-through ssh args after `--`)
  - [ ] `-e, --ephemeral` — connect to an ephemeral instance
  - [ ] `--plan <string>` — plan for the ephemeral instance (only with `--ephemeral`)

## Management

- [ ] **`blueprints`** — manage Blueprints (infrastructure as code)
  - [ ] `blueprints validate <render.yaml>` — validate a Blueprint YAML file
- [ ] **`environments <projectID>`** — list a project's environments
- [ ] **`projects`** — list projects in the active workspace

## Additional commands

- [ ] **`docs`** — open the Render docs in the browser (no API call)
- [ ] **`ea`** — early-access commands (subject to change)
  - [ ] `ea objects` — manage object storage
    - [ ] `--local` — use local storage (`.render/objects/`) instead of cloud
    - [ ] `--region <REGION>` — target region (or `RENDER_REGION`)
    - [ ] `ea objects list` — list objects
    - [ ] `ea objects put <key> --file <path>` — upload an object
    - [ ] `ea objects get <key> --file <path>` — download an object
    - [ ] `ea objects delete <key>` — delete one or more objects
  - [ ] `ea sandbox` — manage sandboxes
    - [ ] `ea sandbox create` — create a sandbox
      - [ ] `--base <string>` — base image
      - [ ] `--plan <starter|standard|pro>` — compute plan
      - [ ] `--region <string>` — region to run in
      - [ ] `--timeout <int>` — max sandbox lifetime in seconds
    - [ ] `ea sandbox exec <sandboxID> -- <cmd>` — run a command in a sandbox
    - [ ] `ea sandbox list` — list sandboxes (`--all`)
    - [ ] `ea sandbox stop <id>` — terminate a sandbox
- [ ] **`skills`** — manage Render agent skills for AI coding tools
  - [ ] `skills install` — install skills
    - [ ] `--tool <claude|codex|opencode|cursor>` — install to a specific tool
    - [ ] `--scope <user|project>` — installation scope
    - [ ] `--skill <skill>` — install specific skills only (repeatable)
    - [ ] `--dry-run` — show what would be installed
  - [ ] `skills list` — list installed skills and detected tools
  - [ ] `skills update` — update previously installed skills
  - [ ] `skills remove` — remove installed skills
- [ ] **`help [command]`** — help about any command
