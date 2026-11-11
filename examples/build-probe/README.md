# build-probe — do a workspace's builds start together?

Acceptance fixture for [w6/m306](../../.pm/w6/done/m306/README.md) (docs/ADR060 §D6, §D8): Render limits builds only per service, so the four services in [`render.yaml`](render.yaml) should all build at once.

Each build sleeps for `PROBE_SECONDS` (420 by default) and does nothing else, so it holds a build slot without loading the build node. Seven minutes outlasts an elastic build node's scale-from-zero, which is what makes the result readable from the tenant side:

| What builds 3 and 4 do | What it means |
| --- | --- |
| start within a minute of 1 and 2 | four warm build slots: aligned |
| start a few minutes in, while 1 and 2 still run | admission let them through and they waited for a node: aligned, with a capacity cost (ADR060 §D8) |
| start only when 1 and 2 finish, about seven minutes in | admission held them: a per-workspace limit is still in force |

A deploy reads `queued` both while it is held and while its build waits for a node, and `build_in_progress` from the moment its build pod is placed. So compare each deploy's `startedAt` (REST `GET /v1/services/{id}/deploys`, GraphQL `deploys`, MCP `list_deploys`) against the others.

## Run it

Apply `render.yaml` in one workspace with `POST /v1/blueprints/deploy` (or the MCP `deploy` tool), prefixing the service names. Read the four deploys' `startedAt`, then delete the four services. The image serves `ok` on port 10000.
