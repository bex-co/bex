# w6 · m148 — Private services serve every port their image declares, as on Render (port slice of deploying `render-examples/clickhouse`)

**Worker:** worker6 **Goal:** in any workspace, a new Git-built Docker private service with no configured port is reachable on every TCP port its image declares. It runs its health check against a port the container can bind, and reports those ports as Render's `openPorts`. Blueprint `fromService` `port`/`hostport` resolve to the port the target actually serves, and none of this grants the ADR089 `image-v1` capability policy. **Status:** done (t001–t006 2026-10-19; t007–t011 2026-10-20, live on production)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Decide the port contract: split ADR089 port discovery from the capability gate, primary-port rule, reference mechanism, rollout switch — **DONE** | 40m | — |
| t002 | Backend: default image-derived ports for new Git-built Docker private services in every workspace; capabilities stay allowlisted — **DONE** | 45m | t001 |
| t003 | Operator: bindable primary selection and a Render-style "no listener on the expected port" diagnosis — **DONE** | 1h | t001 |
| t004 | Read surfaces: Render `openPorts` on REST/MCP `serviceDetails`, GraphQL `Service.openPorts`, dashboard port list — **DONE** | 1h | t002, t003 |
| t005 | Blueprint: `fromService` `port`/`hostport` stop resolving to a parse-time 3000 — **DONE** | 45m | t001 |
| t006 | bex-api: propagate service-port references when a target's serving port changes — **DONE** | 1h | t005 |
| t007 | Fixture + live acceptance: multi-port private service in a customer workspace, unchanged ClickHouse in `bex` — **DONE** | 1h | t002, t003, t004, t005, t006 |
| t008 | Render parity — **DONE** | 20m | t007 |
| t009 | Simplify — **DONE** | 15m | t008 |
| t010 | Test coverage — **DONE** | 40m | t008, t009 |
| t011 | Closeout — **DONE** | 10m | t010 |

## Definition of done

1. **Customer workspace, strict policy.** The test uses a workspace that is **not** in `BEX_IMAGE_COMPATIBILITY_WORKSPACES`. Create a fresh private service from `examples/multi-port` (t007): Dockerfile `EXPOSE 8123 9000`, non-root, ignores `$PORT`, **no port configured**.
   - Its first deploy reaches Live.
   - The running pod still drops `ALL` capabilities and adds none (read from the pod, not the projected object).
   - A web service in the same workspace gets the HTTP response from `http://<slug>:8123/` and the TCP banner from `<slug>:9000`, through the stable hostname.
2. **Read surfaces agree.** REST `GET /v1/services/{id}` and MCP `get_service` return:
   - `serviceDetails.openPorts == [{"port":8123,"protocol":"TCP"},{"port":9000,"protocol":"TCP"}]`
   - `serviceDetails.port == 8123`
   - `serviceDetails.internalAddress == "<slug>:8123"`

   GraphQL `server(id){openPorts port internalAddress}` returns the same values. The dashboard shows both ports beside the internal address.
3. **References follow the real port.** In one Blueprint, a consumer declares `fromService: {type: pserv, name: multi-port, property: hostport}` and another declares `property: port`. On first apply, the consumer's running environment holds `<slug>:8123` and `8123`, never `:3000`. A configured-port service whose port is changed in Settings from 3000 to 8080 is resolved as `<slug>:8080` by its Blueprint consumers. That case is broken on `main` today.
4. **Isolation and legacy are unchanged.** A probe from another workspace to both ports is denied. The private service has no public Ingress. A private service created before the upgrade keeps its `portMode`, its Service ports, and its pod template; no rollout happens on upgrade.
5. **Unchanged ClickHouse.** In the `bex` workspace (`tea-d98210cbbpdc73dcrkvg`, where `image-v1` is already admitted), the byte-for-byte `render-examples/clickhouse@355817b1` Blueprint deploys:
   - a peer gets `Ok.` from `http://<slug>:8123/ping` and a `SELECT 1` result over native `<slug>:9000`;
   - `openPorts` lists 8123, 9000 and 9009.

   A startup failure with a non-port cause (capabilities, disk ownership) is recorded with evidence and filed as a follow-up. The milestone then stays blocked; this item is not waived.

## Render evidence (2026-10-17)

- **Private services detect ports.** [Private services](https://render.com/docs/private-services): "Private services can listen on _almost_ any port … and communicate using any protocol." They also "must bind to at least one port". The limit is "a maximum of 75 open ports", and 18012/18013/19099 "_cannot_ be used for private network communication".
- **API reports them.** The vendored `lego/backend/internal/api/openapi/render-public-api-1.json` makes `openPorts: serverPort[]` **required** on both `webServiceDetails` and `privateServiceDetails`. Its shape is `serverPort {port: integer, protocol: "TCP"|"UDP"}`. There is no writable port field on create or update. bex emits no `openPorts` today: none of its non-test Go code references `openPorts`.
- **Health check.** [Health checks](https://render.com/docs/health-checks): "By default, health checks are TCP socket probes to one of your service's open ports … Private services only support default TCP checks."
- **Deploy failure wording.** [Troubleshooting deploys](https://render.com/docs/troubleshooting-deploys) has the heading "Port scan timeout reached": "Render failed to detect at least one open port on your service's host before its scan timed out." The archived log line is `Timed out: Port scan timeout reached, no open ports detected. Bind your service to at least one port.`
- **Blueprint references.** [Blueprint spec](https://render.com/docs/blueprint-spec) says `port` means "The port of the service's HTTP server", and gives `my-service:10000` as a `hostport` example. Whether Render resolves this from `PORT` or from the detected port is not documented.
- **Web services.** `PORT` defaults to 10000, users may override it, and Render "is _usually_ able to detect" another bound port; one public HTTP port per web service. Web services are out of scope here (see below).

## Mechanism (current `main`, 2026-10-17)

- **One gate for two features.** `configureNewImageCompatibility` (`lego/backend/internal/apps/service.go:2434`) sets two things for new Docker/image services, under the single `imageCompatibilityAdmits` allowlist (`:2451`):
  - `ContainerPolicy = image-v1`;
  - for Git-built private services, `PortMode = image-v1` (or `image-configured-v1` when a port is explicit).

  Production lists only the `bex` workspace (`lego/operator/config/prod/kustomization.yaml:107-113`), so customer private services get no image ports at all. The DB columns are already independent: `0135_app_image_compatibility.up.sql` has separate `container_policy` and `port_mode` CHECKs. Workload admission keys only on the `app.bex.co/container-policy` label (`deploy/gitops/base/operator-workload-admission.yaml:96-110`), not on port mode.
- **Port discovery already exists.** `resolveImageNetwork` (`lego/operator/internal/controller/image_network.go:35`) reads the digest-verified image config of the service's own built repository. `privateTCPPorts` (`lego/operator/internal/registry/image_config.go:178`) drops UDP, rejects reserved ports and caps the count at 75.
- **Primary-port gap.** The primary is 3000 if declared, else `ports[0]`, the lowest declared port (`image_network.go:72`). A port below 1024 is accepted as primary and becomes `PORT`, yet no tenant container can bind below 1024 (ADR022: `NET_BIND_SERVICE` is dropped under both policies). So an image inheriting `EXPOSE 80` while binding `$PORT` would regress from today's 3000.
- **Service projection.** `privateServiceProjection` (`:101`) publishes every resolved port under both stable hostnames, selected to the serving revision. `promoteImageNetwork` (`:123`) runs only after readiness. The readiness probe is TCP on the primary (`deployment_projection.go:344`). The probe-failure diagnosis reads only `a TCP connect to port <n>` (`app_controller.go:5353`).
- **Network policy allows all same-workspace ports.** The workspace default-deny admits same-namespace peers on all ports. The protected-environment policy is peer-based with no `ports` (`environment_network_policy.go`). Multi-port reachability therefore needs no policy change; t007 still verifies it.
- **Blueprint references are frozen at parse time.** Blueprint services are compiled with `Port: 0` (`lego/backend/internal/apps/deploy.go:2659`), so `idx.servicePorts` is always `DefaultPort` (`:2125-2129`). `hostRef.env` formats `<slug>:<parse-time port>` (`:610`), and apply looks up only the slug (`resolveServiceRefs` `:1078`, `patchDeferredStackServices` `:1054`). As a result, every `fromService` `port`/`hostport` resolves to 3000. That includes targets whose port was changed through `SetPort` (`service.go:4527`, shipped w4/m121), so it is already a live bug for configured ports. ADR089 names this (its D4) as the reason customer workspaces stay excluded.
- **Read surfaces publish one number.** `renderServiceDetails` (`render.go:401`, shared by REST and MCP) publishes `port` (`:409`) and `internalAddress` from `addressablePort` → `App.ServicePort()` (`service.go:4557`; `lego/types/v1alpha1/image_network.go`). GraphQL `Service.port` is at `graphql.go:324`. The dashboard `service-port-row.tsx` shows one editable port.

## Contract changes (every artifact moves with the ADR)

m148 changes the ADR041 single-port contract and the ADR089 D3/D4 contract. Each machine-checked artifact that encodes them changes in the same task as the code. An ADR edit without its artifact, or the reverse, is not done.

| Contract | Artifact | Owner |
| --- | --- | --- |
| ADR text | ADR089 D3/D4 and checkpoint (gate split, primary rule); ADR041 (private services may publish many ports; the primary drives `internalAddress`); ADR004 (`PORT` = resolved primary for image-port services); ADR049 (`fromService` port semantics); ADR018 (Internal address row `:97`, `PORT` row `:129` cross-link, new `openPorts` note); `docs/AGENTS.md` one-liners if a summary changes | t001 (decisions), t008 (final parity wording) |
| CRD contract (`lego/types`) | `image_network.go` docs (`UsesImagePorts`, `ServicePort`, `InternalAddress` "D2 resolvability contract"); the typed service-port env reference if t001 picks it; `operator/config/crd/bases/app.bex.co_apps.yaml` + `zz_generated.deepcopy.go` via `make` in `lego/operator` | t005, t006 |
| Store projection | `lego/backend/internal/store` round trip of the new env reference (`store.go`, reconciler); real-Postgres test | t005 |
| REST vs Render OpenAPI | The vendored `render-public-api-1.json` stays byte-identical (it is Render's contract). `api/conformance_allowlist_test.go` `list-services`/`retrieve-service` `serviceDetails: does not match the documented union` currently masks the missing `openPorts`: re-run `TestRenderConformance -v`, then narrow or annotate the entry | t004 |
| GraphQL schema | `apps/graphql.go` `Service.openPorts`; dashboard codegen offline: `SCHEMA_DUMP_PATH=… go test ./internal/api -run TestDumpGraphQLSchema`, then `SCHEMA_JSON=… yarn codegen` (`dashboard/src/graphql/definitions.ts`, `schema-types.ts`) | t004 |
| MCP pin | `api/mcp_parity.go`: check whether `get_service`'s added `openPorts` keeps its classification against upstream; add an accepted-divergence entry only if the output contract departs | t004 |
| Blueprint capability registry | `apps/schema/capabilities.json` `#/definitions/serviceEnvVarProperty/enum` `"port"`/`"hostport"`: handler, fixture (today `TestValidateBlueprintValidYAML`, which never proves the port value) and reason now point at the real-port resolution test. `render.yaml.json` and its `schema.sha256` stay unchanged. The CMS checker's snapshot lives outside this repo: regenerate it from the registry, or record a follow-up | t005, t008 |
| Runtime config contract | `lego/backend/AGENTS.md` env table: `BEX_IMAGE_COMPATIBILITY_WORKSPACES` now gates capabilities only, plus the new rollback switch; the comment at `lego/operator/config/prod/kustomization.yaml:107-109` | t002 |
| Guards that pin today's behavior | `TestImageCompatibilityIsScopedToListedWorkspaces` (`apps/container_policy_test.go:182`) asserts a non-listed workspace gets **no** port mode. It changes deliberately, with a comment pointing to this milestone. ADR041 guards (`operator/.../slug_service_test.go`, `apps/apps_test.go:136`, `apps/stack_test.go:266,532`) keep passing and gain multi-port cases | t002, t003, t010 |

## Source + Goal linkage

- **Source:** the user opened `w6/m148` on 2026-10-17 as "Deploy https://github.com/render-examples/clickhouse to bex". A gap analysis that day found:
  - the legacy `env` alias already works (`TestRenderClickHouseBlueprintUnchanged`);
  - create-time disks already work;
  - the remaining blockers are ADR089's capability policy and image ports, both allowlisted to the `bex` workspace.

  User decision, same day: "先支持 custom port 吧，像是 render 一样" ("support custom ports first, like Render"). This milestone is that port slice. Research: [ADR089](../../../docs/ADR089-render-container-compatibility.md) D3/D4 and its implementation checkpoint, [ADR041](../../../docs/ADR041-service-addresses.md), the [ADR018](../../../docs/ADR018-render-parity.md) `PORT` row (`:129`) and Internal-address row (`:97`), and the Render evidence above.
- **Goal linkage:** ADR008's core hosting pillar ("the open-source Render alternative"). Unchanged Render Docker examples are the migration path (ADR089 customer outcome). Address parity: ADR041, ADR018 Internal address row.
- **Expected outcome:** a Render user's Docker private service that listens on its own ports (8123, 9000, 5432, …) works on bex without a bex-specific port setting, in every workspace. The API reports the ports in Render's shape. Sibling references point at the real port. ClickHouse's network half is proven in the `bex` workspace, so the next milestone is only the capability policy.
- **Why now:**
  - ADR089 D3 is implemented but unreachable for customers, because it is gated together with the separate capability decision.
  - D4 is the named blocker, and its root cause is an existing bug that already mis-addresses configured-port services.
  - Splitting ports from capabilities lets each risk be reviewed separately, which ADR089 D2 asks for. Ports need no security exception.
- **Render parity included:** the milestone changes REST, GraphQL, MCP and the dashboard (`openPorts`, `port`, `internalAddress`, Blueprint reference values).
- **Related, not taken over:**
  - `w4/blocked/m182`: public routing 404s while a configured web port changes; edge Ingress behavior is unchanged here.
  - `w4/blocked/m181`: pre-deploy endpoint isolation.
  - The ADR089 capability policy for customer workspaces is the next ClickHouse milestone.

## Out of scope

- Granting `image-v1` capabilities outside the allowlist (ADR089 D2 security review). Unchanged ClickHouse stays undeployable in customer workspaces until that lands.
- Runtime socket discovery of listeners the image does not declare, UDP, and Render's private-network port-10000 alias (ADR089 D3 non-goals).
- Port detection for web services (the public port stays the configured `port`) and honoring a user `PORT` variable. That stays refused per w2/m95 option (a); revisit only on an explicit user decision.
- Prebuilt `image:` private services (ADR089 "direct prebuilt-image port discovery": arbitrary-registry config reads need their own credential/SSRF review) and native-runtime private services (no image metadata).
- The CMS browser checker / capability snapshot.
