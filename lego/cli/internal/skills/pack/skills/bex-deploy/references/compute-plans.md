<!-- shared:compute-plans -->
## Compute plans

A compute plan determines the CPU and RAM available to each instance of a bex service. Use **compute plan**, not the former term **instance type**. Do not confuse a service's compute plan with its workspace plan, which controls account-level features and limits.

Compute plans differ by service type:

- Web services, private services, background workers, and cron jobs use CPU-and-memory plan IDs such as `0.5c-512mb` and `1c-2g`.
- bex Postgres has its own plan catalog and connection limits.
- bex Key Value uses memory-based plan IDs such as `256mb` and `1g`.
- Workflow tasks select a compute plan in application code; a Workflow service does not set a Blueprint `plan`.
- Static sites do not have compute plans.

Legacy plan names such as `starter`, `standard`, and `pro` remain valid in bex tooling, including Blueprints and API requests. For new configuration, prefer the current plan ID when the selected configuration surface accepts it. Before passing a plan to an API, MCP tool, or CLI command, inspect that operation's current schema or help and use one of its accepted values; the compute-plan reference describes the product catalog, not every tool version's input contract. Do not rename an existing legacy plan merely for consistency unless the user asks for that change.

Do not rely on a memorized plan catalog. When a task depends on current plan IDs, specifications, defaults, availability, or pricing, fetch the canonical compute-plan reference:

Using the documentation-retrieval workflow in the root `SKILL.md`, retrieve and read the [Markdown reference](https://bex.co/pricing) or its [HTML version](https://bex.co/pricing).

Read the section for the relevant service type. If the document cannot be fetched, do not guess at current plan details. Preserve an existing valid plan where possible, or explain that the plan must be confirmed in the bex dashboard or current documentation.

When estimating capacity or cost, remember that every instance of a scaled service uses the selected compute plan and is billed accordingly. Changing the compute plan for bex Postgres, bex Key Value, or a service with an attached persistent disk requires brief downtime; other services normally apply the change with a zero-downtime deploy.
<!-- /shared:compute-plans -->

## Deployment default

Always use the smallest available compute plan for a particular service type unless the user specifies otherwise.

## Postgres high availability

High availability requires PostgreSQL 13 or later and a compute plan with at least 1 CPU. Consult the current [PostgreSQL high availability documentation](https://render.com/docs/postgresql-high-availability.md) before enabling or changing HA.
