# Runbook: run the full agent-session stack locally on `dev-N`

**Status:** written 2026-08-21 (`w3/m79/t001`), executed across `w3/m79`. **Applies to:** any developer who needs the [ADR047](../ADR047-cloud-coding-agent-sessions.md) cloud coding-agent session feature working end to end on the shared local kind/CAPD `bex` cluster via `scripts/dev-env.sh N` — not just the control-plane UI, but a live sandbox whose driver streams a real turn and delivers a draft PR.

## Why this exists

`scripts/dev-env.sh N up` stands up bex-api + Kratos/Hydra/Mailpit + CNPG + Loki, but it wires **none** of the agent-session substrate. The feature is hard-gated in `lego/backend/internal/agentsessions/service.go:398`: every create/steer/attach path returns `core.ErrAgentSessionsUnavailable` unless all three hold:

- `enabled()` (`service.go:325`) — `Store != nil && Tuples != nil && Sandbox != nil` → needs the CP store (set), **OpenFGA**, and an **OpenSandbox** client.
- `modelProxyEnabled()` (`service.go:329`) — `BEX_AGENT_MODEL_PROXY_URL` non-empty → needs the in-cluster **ssh-gateway** model proxy.
- `ticketEnabled()` (`service.go:331`) — `BEX_SHELL_TICKET_SECRET` **and** `BEX_AGENT_SESSION_GATEWAY_URL` → needs the gateway attach listener + a shared secret.

Beyond the gates, the create/dispatch path additionally needs **OpenBao** (the model-credential mint handler is wired only when `BEX_OPENBAO_URL` is set — `cmd/api/main.go:977` — and reads the BYO key from `agent-sessions/<ws>/model-key`), the **Cilium CRD** (sandbox-create installs a `CiliumNetworkPolicy` egress rule — `sessionegress/policy.go:55` — and CAPD runs Calico), a real **GitHub App** installation (clone + draft PR), and the **agent image** (`lego/agent-image/Dockerfile`) available to the sandbox pods.

## The load-bearing fact: the gateway must run in-cluster

The ssh-gateway cannot run as a host process locally. Two of its four sandbox-facing transports are structurally unreachable from the host on CAPD/OrbStack (Calico pod IPs are not routed onto the host — see `scripts/mock-cluster.sh:50-54`, `scripts/dev-env.sh:191-201`):

| Transport | Port | Data path | Host-side? |
| --- | --- | --- | --- |
| Agent attach (browser SSE + turn) | 8083 | gateway → **podIP:8787** direct (`agentattach/agentattach.go:162,688`) | **no** |
| Model proxy | 8084 | **sandbox pod → gateway** via cluster DNS (`service.go:1271`) | **no** |
| Git proxy | 8082 | **sandbox pod → gateway** via cluster DNS (`service.go:1233,1250`) | **no** |
| Sandbox-exec / native SSH | 8081 / 2222 | gateway → apiserver `pods/exec` (`cmd/ssh-gateway/main.go:150`) | yes |

So the gateway is deployed as a Deployment in the `bex` workload cluster, exactly like prod (`lego/operator/config/ssh/`). The one genuinely novel local hop is the **reverse** one: the in-cluster gateway must reach the **host** bex-api's `:8091` control-plane listener to mint git/model credentials (`cmd/ssh-gateway/main.go:229,243`).

## Addressing map (local)

| Hop | Client | Local wiring |
| --- | --- | --- |
| browser → gateway attach `:8083` | dashboard (host) | port-forward; `BEX_AGENT_SESSION_GATEWAY_URL=http://localhost:<pf8083>` |
| bex-api → gateway exec `:8081` | bex-api (host) | port-forward; `BEX_SANDBOX_EXEC_URL=http://localhost:<pf8081>/sandbox-exec` |
| sandbox pod → gateway git/model `:8082/:8084` | pod | in-cluster Service DNS (host-opaque, pod-resolved); handed to bex-api as `BEX_AGENT_GIT_PROXY_URL` / `BEX_AGENT_MODEL_PROXY_URL` |
| gateway → sandbox pod `:8787` / apiserver exec | gateway (in-cluster) | pod network / ServiceAccount — no Cilium `:8787` gate locally (Calico) |
| gateway → bex-db | gateway (in-cluster) | Service DNS `bex-db-rw.<ns>.svc:5432` |
| **gateway → host bex-api `:8091`** | gateway (in-cluster) | ExternalName/Endpoints to the OrbStack host on the dev-N `BEX_CP_PORT`; `BEX_AGENT_CREDENTIAL_API_URL` / `BEX_AGENT_MODEL_CREDENTIAL_API_URL` |
| OpenSandbox server → host bex-api `:8091/v1/sandbox-tenants` | OpenSandbox (host) | tenant-key → `<ws>-sandbox` resolution (`store/api.go:139,548`) |

## Component inventory (what the overlay stands up, on top of `dev-env.sh N up`)

1. **OpenFGA** — `openfga/openfga:latest run` (in-memory), seeded via `scripts/authz-model.sh` (store `bex` + model + bootstrap tuple). bex-api resolves store `bex` by name (`authz.go:236`). Setting `BEX_OPENFGA_URL` on bex-api means dropping `BEX_ALLOW_INSECURE_AUTHZ` — authz becomes real, so the dev user must be a workspace member (the normal create-workspace flow writes that tuple). The gateway **requires** `BEX_OPENFGA_URL` (`main.go:67`, no insecure bypass).
2. **OpenBao** — dev-mode (auto-unsealed) server; `BEX_OPENBAO_URL` on bex-api; store the real provider key at `agent-sessions/<ws>/model-key` field `BEX_AGENT_MODEL_API_KEY` (`agentsession/model.go:114`).
3. **Cilium CRD** — apply only the `CiliumNetworkPolicy` (cilium.io/v2) CRD so `sessionegress.Manager.Create` succeeds. The policy is inert under Calico (no enforcement) — acceptable for local dev; the isolation guarantee is simply not exercised locally.
4. **OpenSandbox controller + BatchSandbox CRD** — `helm upgrade --install opensandbox-controller deploy/gitops/charts/opensandbox-controller -n opensandbox-system --create-namespace -f deploy/gitops/base/values/opensandbox-controller.values.yaml`. Pin the manager to the control-plane node (CAPD workers can't reach the apiserver — same constraint CNPG hits, `dev-env.sh:257`).
5. **OpenSandbox k8s-runtime server (host)** — `uvx --from opensandbox-server==0.2.2 opensandbox-server --config <local-multitenant.toml>` pointed at the CAPD kubeconfig, with a `[tenants]` block → `http://<host>:<BEX_CP_PORT>/v1/sandbox-tenants` (so pods land in `<ws>-sandbox`), no `[secure_runtime]` block, and `batchsandbox_template_file` → a **relaxed** local template (drop `runtimeClassName: gvisor` and the `bex.co/pool=sandbox` nodeSelector; label a CAPD worker `bex.co/pool=sandbox` if you keep the selector). Do not edit the tracked `deploy/opensandbox/batchsandbox-template.yaml`; point at a generated copy.
6. **Agent image** — `docker build -f agent-image/Dockerfile -t bex-agent-sandbox:dev lego/` then import into each CAPD node (`ctr -n k8s.io images import`, the ADR004 §3 pattern) or push to the in-cluster Zot; set `BEX_AGENT_SESSION_IMAGE` on bex-api to the imported ref with `imagePullPolicy: IfNotPresent`.
7. **ssh-gateway (in-cluster)** — build the shared image's `ssh-gateway` entrypoint, import into CAPD nodes, deploy Deployment + ServiceAccount + RBAC (`pods` get, `pods/exec`) + host-key Secret (Ed25519) + `bex_ssh_gateway` DB role (`scripts/ssh-gateway-db-role.sh`) + Service (2222/8080-8084/9090), wired with `BEX_OPENFGA_URL`, `BEX_CP_DB_URI` (in-cluster DNS), and `BEX_SHELL_TICKET_SECRET` / `BEX_SANDBOX_EXEC_SECRET` **byte-matching** host bex-api, plus the reverse-hop mint URLs. Omit the restrictive ingress NetworkPolicy locally (or label the sandbox namespace `app.bex.co/regime=sandbox`).
8. **bex-api env injection (host)** — `BEX_OPENSANDBOX_URL`, `BEX_SANDBOX_EXEC_SECRET`, `BEX_SANDBOX_EXEC_URL`, `BEX_SHELL_TICKET_SECRET`, `BEX_AGENT_SESSION_GATEWAY_URL`, `BEX_AGENT_MODEL_PROXY_URL`, `BEX_AGENT_GIT_PROXY_URL`, `BEX_AGENT_SESSION_IMAGE`, `BEX_OPENFGA_URL` (+ drop `BEX_ALLOW_INSECURE_AUTHZ`), `BEX_OPENBAO_URL`, and a **real** `BEX_GITHUB_APP_ID/_PRIVATE_KEY/_SLUG` (the dev stub id `1` cannot clone).

## Bring-up order

OpenFGA + OpenBao → Cilium CRD → OpenSandbox controller/CRD → agent image import → gateway image + Deployment → reverse-hop Service + port-forwards → host OpenSandbox server → bex-api env injection → `dev-env.sh N status` (agent checks) → end-to-end smoke.

## Verify

- `capabilities.enabled == true` — probe the agent-session capabilities (REST/GraphQL `Capabilities`, `service.go:537`). All three gates passed.
- Create a session against a repo on the real GitHub App installation; confirm the sandbox pod reaches `Ready` in `<ws>-sandbox`, the attach stream emits `data-acp` parts, one turn runs against the model proxy, and delivery opens a draft PR.

## Real credentials required (developer-supplied, never committed)

- A model provider key (e.g. `ANTHROPIC_API_KEY`) → stored in local OpenBao at `agent-sessions/<ws>/model-key`.
- A GitHub App with an installation on a disposable repo → `BEX_GITHUB_APP_ID` / `_PRIVATE_KEY` (PEM) / `_SLUG`.

## Live bring-up gotchas (discovered running this on 2026-08-21)

These bit during the first real bring-up and are not obvious from the code:

- **Approve kubelet-serving CSRs.** `mock-cluster.sh` sets `rotate-server-certificates: true` but has no serving-cert approver, so every `port-forward`/`exec`/`logs` fails with `tls: internal error` until you `kubectl certificate approve` the `kubernetes.io/kubelet-serving` CSRs. Do this right after the cluster is Ready.
- **Build the OpenSandbox controller for arm64.** The pinned `ghcr.io/bex-co/opensandbox-controller` image is amd64-only (`no matching manifest for linux/arm64`). On Apple Silicon, build it from `deploy/opensandbox/controller.Dockerfile` (native `TARGETARCH`) and `ctr -n k8s.io images import` it into the node; patch the deploy to the local tag + `IfNotPresent`.
- **Create the tenant-namespace ClusterRoles.** The `NamespaceReconciler` creates RoleBindings in each `<ws>-sandbox` (`bex-tenant-sandbox-controller`, `bex-tenant-sandbox-server`, `bex-tenant-ssh-gateway`, `bex-operator-snapshot-pull`) but the **ClusterRoles they reference are gitops artifacts** absent locally — so the controller gets `pods is forbidden … clusterrole "bex-tenant-sandbox-controller" not found` and no pod is created. Apply those ClusterRoles (at minimum `bex-tenant-sandbox-controller` with pods CRUD) into the cluster.
- **Reverse hop address.** From a CAPD pod, the host is reachable as `host.docker.internal` (OrbStack) — `192.168.<net>.1` (docker gw) is not. Point the gateway's mint URLs at `http://host.docker.internal:<BEX_CP_PORT>`.
- **Co-locate on the control-plane node.** Under OrbStack+Calico, pin the gateway **and** the sandbox template to the control-plane node so the `podIP:8787` dial is same-node (cross-node pod routing is unreliable). The control-plane node carries a `NoSchedule` taint, so both need the toleration.
- **The plain `POST /v1/sandboxes` API call blocks** because the host server's `ingress = "direct"` waits to reach the pod's execd at its pod IP (unroutable from the host). This does **not** affect agent sessions (the in-cluster gateway reaches the driver); the sandbox pod still schedules and runs.
- **Grant the controller BatchSandbox status RBAC.** The tenant `bex-tenant-sandbox-controller` ClusterRole must include `batchsandboxes` + `batchsandboxes/status` (+ `sandboxsnapshots`, `pools`) patch/update, or the controller creates the pod but can't write the sandbox status back — OpenSandbox then never sees it go Running and times out at `POD_READY_TIMEOUT` (300s) even though the pod is healthy.
- **Egress is BLOCKED, not open, without Cilium — and it's PER workspace namespace.** Every `<ws>-sandbox` namespace carries a Calico-enforced `default-deny` NetworkPolicy; the matching ALLOW is a `CiliumNetworkPolicy` that Calico ignores. So the sandbox can't reach the gateway model/git proxy, and the turn fails after a long hang — the session lands **Failed** with `Internal error: Request timed out (code -32603)` (the model call never connects). The fix is a plain k8s `NetworkPolicy` allowing egress, and it must exist in **each** tenant sandbox namespace — a namespace is created per workspace, so every new workspace needs it (this is exactly why one workspace's sessions work while a freshly-created workspace's all fail with -32603). Apply to all existing sandbox namespaces (re-run after creating a workspace):

  ```bash
  for ns in $(kubectl get ns -o name | sed 's|namespace/||' | grep -E '^tea-.*-sandbox$'); do
    kubectl apply -n "$ns" -f - <<'YAML'
  apiVersion: networking.k8s.io/v1
  kind: NetworkPolicy
  metadata: { name: dev-allow-egress }
  spec: { podSelector: {}, policyTypes: [Egress], egress: [{}] }
  YAML
  done
  ```

  (local-dev only; the FQDN egress restriction is simply not exercised without Cilium.)

- **The conversation stream has no local edge route.** In prod `api.bex.co/v1/agent-sessions/{id}/stream` is edge-routed to the gateway; the dashboard builds the stream URL from its API base assuming that. Locally there is no edge, so (a) set `BEX_API_PUBLIC_URL` on bex-api to the gateway attach origin (populates the ticket's `streamUrl`), and (b) point the dashboard at the gateway for the stream with `VITE_AGENT_STREAM_URL=http://localhost:<pf8083>` (config `agentStreamBaseUrl`, defaults to `apiBaseUrl` so prod is unchanged). The gateway's attach listener already CORS-allows the dashboard origin + the `x-bex-agent-ticket` header.
- **The plan ResourceQuota caps a free workspace at ~2 sandboxes.** A `hobby`/free workspace's `<ws>-sandbox` `tenant-quota` is `quotaForPlan(hobby)` = `limits.cpu: 4` / `requests.cpu: 2` (`store/namespaces.go`), i.e. **exactly two** live sandboxes (2cpu each). Old sessions' sandboxes aren't reaped promptly (30m idle TTL), so once two accumulate a new session **fails**: the dashboard shows `Session failed / sandbox create failed` and bex-api logs `the sandbox did not become ready within the platform's wait; the likely cause is this workspace's sandbox capacity (plan limit)`. Two remedies:
  - **Durable (preferred):** upgrade the workspace's plan so the projector reprojects the generous paid quota (`limits.cpu: 100`, ~50 sandboxes) — a local data change, no billing locally. `psql "$BEX_CP_DB_URI" -c "UPDATE tenants SET plan='pro' WHERE id='<ws>'"`; the projector reconciles the quota within ~30s (it reverts any direct `kubectl patch` to the plan value, so patch the plan, not the quota).
  - **Quick:** free capacity with `kubectl -n <ws>-sandbox delete bsbx --all` (use the `bsbx` short name / `batchsandboxes.sandbox.opensandbox.io` — the singular `batchsandbox` intermittently fails API discovery here). The controller backs off exponentially, so a sandbox already stuck won't retry the instant quota frees — nudge it: `kubectl -n <ws>-sandbox annotate bsbx <id> bex.co/nudge="$(date +%s)" --overwrite`.

## Known local divergences from prod

- No Cilium: the `:8787` gateway-only ingress and the per-session FQDN egress are **not enforced** locally (Calico); the `CiliumNetworkPolicy` objects are created but inert. Do not treat a local run as isolation validation.
- No gVisor: sandbox pods run under `runc` on a CAPD node, not the runsc sandbox pool.
- Hibernation (`BEX_AGENT_SNAPSHOT_S3_*`) is left off; the Active tier is sufficient for local dev.
