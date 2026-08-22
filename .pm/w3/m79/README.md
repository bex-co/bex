# w3 · m79 — Run the full agent-session stack locally on `dev-N`

**Worker:** worker1 **Goal:** a developer can bring the entire [ADR047](../../../docs/ADR047-cloud-coding-agent-sessions.md) agent-session stack up end to end on the shared local kind/CAPD `bex` cluster — live sandbox, streamed turn, draft PR — via one opt-in path on `scripts/dev-env.sh N`. **Status:** todo

## Tasks (in order)

| id   | title                                                                            | est | depends_on            |
| ---- | -------------------------------------------------------------------------------- | --- | --------------------- |
| t001 | Dev auth/secret services: OpenFGA + OpenBao for `dev-N`                           | 45m | —                     |
| t002 | Cilium CRD + OpenSandbox controller/CRD + relaxed local template into CAPD        | 45m | —                     |
| t003 | Host OpenSandbox k8s-runtime server + multi-tenant `<ws>-sandbox` resolution      | 45m | t002                  |
| t004 | Agent image: build + import into CAPD nodes                                       | 30m | —                     |
| t005 | In-cluster ssh-gateway: image + Deployment/SA/RBAC/host-key/DB-role/Service       | 60m | t001, t002            |
| t006 | Reverse hop (gateway → host bex-api `:8091`) + `:8081`/`:8083` port-forwards      | 30m | t005                  |
| t007 | `dev-env.sh N agents` overlay: orchestrate the above + inject bex-api env         | 60m | t001, t003, t004, t006 |
| t008 | `status`/verify agent stack + end-to-end create-session smoke                     | 45m | t007                  |
| t009 | Simplify                                                                          | 20m | t008                  |
| t010 | Test coverage: `dev-env.test.sh` derivation for the new ports/env                 | 30m | t008                  |
| t011 | Closeout                                                                          | 10m | t009, t010            |

## Definition of done

`bash scripts/dev-env.sh N up --agents` (or the equivalent subcommand) brings up, on top of the base env, OpenFGA + OpenBao + the Cilium CRD + the OpenSandbox controller/CRD + a host OpenSandbox k8s-runtime server + an in-cluster ssh-gateway, and injects the full agent-session env into the locally-run bex-api so all three gates flip. `dev-env.sh N status` asserts `capabilities.enabled == true` and gateway/OpenSandbox/OpenFGA health. With a developer-supplied model key (in OpenBao at `agent-sessions/<ws>/model-key`) and a real GitHub App installation, creating a session lands a sandbox pod in `<ws>-sandbox`, the attach stream emits `data-acp` parts, one turn runs against the model proxy, and delivery opens a draft PR. The runbook `docs/runbooks/agent-sessions-local-dev.md` matches the shipped behavior.

## Source + Goal linkage

- **Source:** user request 2026-08-21 ("how to ensure we can run the entire agents session stack locally with dev-5") + the full-stack trace recorded in `docs/runbooks/agent-sessions-local-dev.md`.
- **Goal linkage:** pillar 5 (ADR047 cloud coding-agent sessions). Today the feature is only exercisable against prod (`scripts/agent-session-verify.sh`); this gives every workstream a local inner loop for agent-session/dashboard/transcript work.
- **Expected outcome:** the agent-session gates (`enabled`/`modelProxy`/`ticket`) flip on `dev-N`, and a session runs to a draft PR against a local sandbox — provably, via `status` + smoke.
- **Why now:** the dashboard agent-session surface (routes `agents`, `agents_.$agentSessionId`) and the transcript/hibernation/archive work (ADR051/059/065) have no local test path, so every change is validated blind or against prod. This closes that gap.
- **Render parity task omitted:** this milestone changes only local dev tooling (scripts, dev manifests, a runbook) — no REST/GraphQL/MCP/UI surface changes — so the standing Render-parity task does not apply.

## Notes / risks

- The ssh-gateway **must** run in-cluster locally (attach dials `podIP:8787`; git/model proxies are pod-initiated over cluster DNS — neither routable from the host on CAPD). See the runbook's placement table.
- The one novel local hop is in-cluster gateway → **host** bex-api `:8091` (credential/model mint), the mirror of dev-env's existing host→cluster port-forwards.
- Real credentials are developer-supplied and never committed: a model provider key (OpenBao) and a GitHub App installation.
- Local divergences from prod are intentional and documented: no Cilium enforcement (Calico), no gVisor (runc). A local run is **not** isolation validation.
