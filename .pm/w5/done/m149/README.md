# w5 · m149 — `PORT` is user-settable like on Render

**Worker:** worker5 **Goal:** a service's own `PORT` sets the port it serves and is probed on, as on Render, instead of being refused. **Status:** done

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Amend ADR018 row 138 and ADR004: PORT is user-settable as on Render — **DONE** | 30m | — |
| t002 | Drop the reserved-PORT refusal on every write path — **DONE** | 1h | t001 |
| t003 | The operator serves, routes and probes on the effective PORT — **DONE** | 1h45m | t001 |
| t004 | Dashboard editors accept PORT; envtests for own, group and precedence — **DONE** | 1h | t002, t003 |
| t005 | Render parity — **DONE** | 30m | t004 |
| t006 | Simplify — **DONE** | 20m | t005 |
| t007 | Test coverage — **DONE** | 45m | t005, t006 |
| t008 | Closeout — **DONE** | 10m | t007 |

Total: **8 tasks, about 6h**.

## Definition of done

- Setting `PORT=8080` on a service rolls it out serving and passing health checks on 8080.
- A linked group's `PORT` behaves as on Render, and every surface accepts the key.

## Source + Goal linkage

- **Source:** fifth `/pm-brainstorm for w5` (2026-10-20), all five items chosen by the user the same day after `2163b27c7` made Render parity the top principle (a divergence stands only with ADR018-recorded user sign-off; never park parity on an optional decision); ADR018 row 138 "Deliberate divergence (w2/m95)".
- **Goal linkage:** ADR008 Render parity first; ADR006/ADR018.
- **Expected outcome:** apps that set their own PORT migrate without edits.
- **Why now:** `2163b27c7` lets a divergence stand only with recorded user sign-off, and this is the most visible unsigned one.
- **Render parity:** included: removes refusals on REST/GraphQL/MCP and the dashboard.

## Closeout (2026-10-21)

- **Docs.** ADR018's `PORT` row is now ✅ on every surface, replacing w2/m95's reservation, which had no recorded sign-off. ADR004 and ADR049 are updated to match.
- **Write paths.** Every one accepts `PORT`: REST, env groups, GraphQL, MCP, create and Blueprint. The `core` reserved-key mechanism is gone, and acceptance tests in `secrets`, `envgroups` and `apps` replace the refusal tests.
- **Operator.** `env_port.go` resolves the effective `PORT` in Kubernetes' precedence: a literal (a Secret reference resolves through its key), then the `envFrom` Secrets, where the service's own set beats a group's. It runs before the cancel branch.
  - Container port, Service, probes and the injected `PORT` all follow the value, which is recorded as `status.envPort`.
  - `App.ServingPort()` prefers it, so `ServicePort`, `OpenPorts`, `InternalAddress` and the Traefik metrics and activity labels follow it too. Image-port services keep their discovered ports.
  - An unusable value is ignored and logged.
- **Dashboard.** The env editors no longer block `PORT`. The Settings port help text says a `PORT` variable overrides the setting.
- **Tests:**
  - Envtests (`env_port_test.go`, `app_controller_test.go`): a literal, a group's Secret, own beating group, an unusable value ignored, and removal reverting.
  - `types` `TestServingPortPrefersTheServicesOwnPORT`.
  - The full operator, backend, types and dashboard suites pass, and lint is clean.
- **Corpus.** It went from 71 to 83 of 170 valid. Two masked gaps were filed as w5/171 (registry allowlist) and w5/172 (`dockerContext: /`), and both are tracked in `knownBlueprintGaps`.
- **Known behavior.** A `PORT` change rolls twice: first still on the old port, then on the new one. The bound port and the routed port never disagree.
- **Deviation from the DoD.** dev-5 reuses the shared cluster's operator, so "PORT=8080 serves and passes probes on 8080" is proven in envtest, not live.
