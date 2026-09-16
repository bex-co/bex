# Router dashboard bridge

Router is an optional, session-only dashboard beta for `tea-d98210cbbpdc73dcrkvg`. The dashboard's `dashboard-router` GrowthBook definition gates navigation and routes. The backend independently checks the tenant, verified Kratos session, current membership role, and fresh OpenFGA authorization before each upstream call.

Configure `BEX_ROUTER_URL` with the companion's GraphQL endpoint and `BEX_ROUTER_ASSERTION_SECRET` with the same signing key used by the companion's BIA verifier (at least 32 bytes). The current BlockEden companion wires that verifier to its existing `AUTH_SECRET` through `setup/service.ts`; match the deployed value, not an independently generated bridge key. A local companion `.env` is not evidence of the deployed configuration. Leave the URL unset to hide Router. The endpoint must use HTTPS or a trusted private HTTP connection; redirects are refused. The signing key stays on the server, and upstream errors are never forwarded.

The public integration subset is [contract.graphql](contract.graphql). The companion's `myWorkspaces` query resolves the assertion's mapped workspace; bex never substitutes a default workspace or accepts a BlockEden workspace ID from the browser. To show the existing BlockEden workspace `ws_iyUG9p5zQHM3E7VDnBQn`, its companion-side mapping must bind it to `workspace:tea-d98210cbbpdc73dcrkvg`. Check that mapping before enabling the integration: the companion can otherwise provision a new empty workspace.

Usage is polled every 30 seconds. API keys are requested without Apollo caching. Creation, rename/CORS updates, and deletion stay on the companion's existing operations. Updates and deletion first check that the key belongs to the mapped workspace. Router subscriptions, RPC endpoints, and existing access keys keep their current behavior.

This beta exposes GraphQL only: there is no Render REST/MCP counterpart, and machine/OAuth credentials cannot mint BIA assertions through it. The operation scope matrix still classifies key creation as minting and key reads as sensitive.
