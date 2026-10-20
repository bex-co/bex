# CLI sweep 1107 — MCP create_web_service lands in REST (live, 2026-10-20)

One disposable free web service created via MCP and deleted via REST (204; GET 404). No new bug filed.

- A first call with a REST-shaped `imagePath` argument was rejected by schema validation (`unexpected additional properties ["imagePath"]`); the tool's own schema (`name` + `runtime` required; `image` string) is the contract. With `runtime:"image"`, `image:`, `plan:"free"`, and `workspaceId`, creation succeeds and REST immediately reads the service. MCP creation is a full peer of REST/GraphQL/CLI creation.
