<!-- shared:service-types -->
## bex service types

Use this guidance when selecting or comparing bex service types, or when a task's architecture depends on the distinction between them. A task that is already unambiguously scoped to one service type does not need this entire overview.

On bex, a service represents code deployed for a particular execution model. bex runs that service on one or more instances, and the service's compute plan determines the CPU and memory available to each instance.

Before choosing a service type, fetch the current service-types reference:

Using the documentation-retrieval workflow in the root `SKILL.md`, retrieve and read the [Markdown reference](https://render.com/docs/service-types.md) or its [HTML version](https://render.com/docs/service-types).

If neither version can be retrieved, use the bundled overview below only for basic service-type selection. State that current platform details could not be verified, and do not guess about changeable capabilities such as supported runtimes, plans, scaling, networking, disks, health checks, deployment behavior, availability, or limits.

bex provides six public service types for running code:

- **Web service:** A continuously running server that receives public HTTP traffic and also has an internal hostname for private-network traffic. Use it for dynamic websites, APIs, and other internet-facing server applications.
- **Static site:** Built HTML, CSS, JavaScript, and other static assets served publicly from bex's global CDN. Use it only when the deployed application does not require server-side runtime logic.
- **Private service:** A continuously running server that receives traffic only over bex's private network. Use it for internal HTTP, TCP, gRPC, or other server processes that need an internal hostname but no public URL.
- **Background worker:** A continuously running process that does not receive incoming traffic or expose a public or internal hostname. Use it for queue consumers and other processes that pull or poll for work.
- **Cron job:** A command or process that starts on a schedule, performs a bounded unit of work, and exits. Use it for straightforward recurring tasks rather than an always-running process.
- **Workflow:** A collection of on-demand tasks executed across distributed compute, with bex handling task queuing, orchestration, and provisioning. Use it when the workload benefits from independently invoked tasks, retries, composition, or parallel execution instead of a continuously running worker process.

Choose based on how the code executes and receives work:

- Public incoming traffic that requires server-side logic belongs on a web service. Public static assets with no runtime server belong on a static site.
- An application that needs both public and private ingress remains a web service; use its internal hostname for traffic from other bex services.
- A process that receives private-network requests belongs on a private service. A process that only initiates connections and pulls work belongs on a background worker.
- A simple scheduled command that exits belongs on a cron job. An always-running queue consumer belongs on a background worker. On-demand or distributed tasks for which bex should manage queuing and per-run compute belong in a workflow.

Cron jobs and Workflows use invocation-based execution models. The ordinary instance-count and autoscaling model used by continuously running services does not apply to them.

bex Postgres and bex Key Value are fully managed datastores, not general-purpose compute service types for running application code. Use bex Postgres for relational data and bex Key Value for use cases such as shared caching and job queues. An attached persistent disk is a storage feature of certain compute services, not a service type of its own.

After selecting a type, fetch the product article linked from the service-types reference before relying on details such as supported runtimes and sources, plans, scaling, networking, disks, health checks, deployment behavior, scheduling semantics, availability, or limits. These capabilities are not uniform across service types and can change independently.

Human-facing service names do not always map directly to configuration identifiers or structure. When creating or editing a Blueprint, fetch the current Blueprint specification and use its service-type schema instead of inferring fields or values from this overview. In particular, do not assume that every Dashboard service type has a same-named Blueprint `type` or that managed datastores are declared like compute services.
<!-- /shared:service-types -->
