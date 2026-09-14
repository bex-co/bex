# w5 · m98 — Live-verification sweep on dev-5

**Worker:** worker5 **Goal:** the five residuals that closed w5 milestones recorded as "deferred — cluster unavailable" (and that nothing on the board owns) each get a dated PASS or FAIL line from a live run, with every FAIL fixed in-task when small or filed as an inbox note carrying `file:line`. **Status:** done

## Tasks (in order)

| id   | title                                                                                                      | est | depends_on             |
| ---- | ---------------------------------------------------------------------------------------------------------- | --- | ---------------------- |
| [t001](done/t001.md) — **DONE** | Bring up `dev-5`, reprovisioning the shared kind/CAPD cluster if needed; record what was rebuilt            | 30m | —                      |
| [t002](done/t002.md) — **DONE** | m89–m91: multi-replica Metrics walk — instance selection, aggregation, percentages, selection persistence    | 45m | t001                   |
| [t003](done/t003.md) — **DONE** | m88: agent-turn latency/outcome series via a repo-less session, or record the model-key blocker             | 45m | t001                   |
| [t004](done/t004.md) — **DONE** | 053: confirm the production availability board's edge 5xx log panel shows platform-host request lines       | 15m | —                      |
| [t005](done/t005.md) — **DONE** | m65 / m66: `agent-session-verify.sh` legs incl. 1b and a real ACP turn, or record the GitHub-App blocker    | 30m | t001                   |
| [t006](done/t006.md) — **DONE** | Render parity                                                                                              | 30m | t002, t003, t004, t005 |
| [t007](done/t007.md) — **DONE** | Simplify                                                                                                   | 20m | t006                   |
| [t008](done/t008.md) — **DONE** | Test coverage                                                                                              | 30m | t006                   |
| [t009](done/t009.md) — **DONE** | Closeout                                                                                                   | 15m | t008                   |

## Definition of done

- Every residual named in t002–t005 has a dated **PASS** or **FAIL** line in this README's Evidence section with the exact command/verb, the surface exercised, and the observed result; "reasoned from code shape" is not evidence. A residual that is blocked by credentials this environment cannot supply gets a **BLOCKED** line naming the exact missing input (same standard as `w3/m79` t012).
- Every **FAIL** is fixed inside its task (≤ ~30m, with a regression test) or filed as a `w5/NNN` inbox note carrying `file:line`, and the source milestone's deferred bullet is annotated with the outcome and pointer.
- No w5 closeout under `done/` still reads "deferred", "not verified live", or "awaiting live verification" without an annotation pointing here.

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w5` 2026-09-10 proposal 6 — a sweep of w5 closeouts: `done/m89/README.md:59` ("Live multi-replica walk deferred when cluster unavailable"), `done/m88/README.md:58` ("live dev-5 registry/panel walk deferred (kubectl/dev-5 unavailable at closeout)"), `done/m65/README.md:58` ("`scripts/agent-session-verify.sh` legs — including the new 1b — need a real cluster + GitHub App and have not been run"), `done/m66/README.md:26` ("Live verification (a real `claude-code-acp` turn)…"), and `done/053.md` ("awaiting live verification by the coordinator"). None of these is among the ten residuals `w1/m139` covered (dropped 2026-09-14).
- **Facts verified 2026-09-10:** the local kind cluster's API refused connections at 21:06 local (`infra/local/bex.kubeconfig`, `127.0.0.1:55003`); `w1/m139`'s evidence records the control plane crash-looping on etcd latency on 09-09 and a green `mock-cluster.sh` re-run on 09-10, so bring-up may again need reprovisioning (pre-approved for `dev-N` and `mock-cluster.sh` per root `CLAUDE.md`). Carried over from `w1/m139`'s 2026-09-12 partial walk before it was dropped:
  - host-wide OOM on the 16 GB OrbStack VM, not a harness bug, was the blocker every time;
  - every `dev-env.sh up` re-seeds the bootstrap secret, so re-mint the token after each run;
  - there is no `default` tenant, so the FGA tuple must name the real workspace (`authz-model.sh` seeds `workspace:default`);
  - Blueprint `image:` is an object, and whoami needs `WHOAMI_PORT_NUMBER: "3000"`;
  - the mock cluster has no Traefik CRDs, so public Apps never reach Ready there and `.status.url` points at prod — use a port-forward.
- **Goal linkage:** pillar 1 correctness of already-shipped work (Metrics page parity, agent-session reliability, ADR088 incident explainability); the board's own rule that a milestone lands in `done/` when its observable end state is real.
- **Expected outcome:** the deferred-verification debt in w5 stops compounding; any defect hiding behind "identical code shape" surfaces with evidence; the 053 panel is proven to explain platform-API incidents.
- **Why now:** each new closeout since 2026-08-20 has added to this list, and `w1/m139` was then doing the same sweep for its own residuals (dropped 2026-09-14, so there is no shared bring-up cost). Honest caveat: the cluster is down at filing time, so this ranks below the shipping milestones (m94–m97).
- **Render parity included:** in-task fixes may touch the dashboard Metrics UI or the metrics/events API; t002 explicitly compares the dashboard read against REST/GraphQL/MCP for the same window.

## Evidence

### t001 — PASS (2026-09-11). Cluster and `dev-5` rebuilt from nothing.

The shared kind/CAPD cluster did not exist at all when this milestone started (API refusing on `127.0.0.1:55003`, no `bex-*` node containers). `scripts/mock-cluster.sh` brought it back: three nodes Ready (`bex-8ng2b-x4qh9` control-plane, `bex-tenant-0-…`, `bex-worker-0-…`), reusing the existing 12-day-old node containers rather than reprovisioning them. `scripts/dev-env.sh 5 up` then came up clean — Kratos, Kratos admin, Hydra admin, Hydra public and bex-api all answering 200, and the verification inventory **ALL GREEN** (default StorageClass, CNPG operator, Loki, bex-api). `bex-api` was rebuilt from the current tree at 00:27. No other workstream's stack was touched; `dev-8` namespaces were observed running concurrently and left alone.

### t004 — PASS (2026-09-11). The platform edge-host log retention is real in production.

Inbox note `053` extended the log-shipper allowlist so the availability board's 5xx log panel can explain platform-API incidents, and closed with "awaiting live verification by the coordinator". Verified against production Loki (port-forward, `{type="request"}` over 24h): **all five** hosts return retained request lines — `api.bex.co`, `dashboard.bex.co`, `oauth.bex.co`, `auth.bex.co`, `obs.bex.co`. The panel now has data to show for exactly the incidents it exists for.

### t002 — PASS (2026-09-14). Multi-replica metrics walk on `dev-5` after installing metrics-server.

**Enabling work first:** installed metrics-server on the mock CAPD cluster (`kubectl top nodes` green; `v1beta1.metrics.k8s.io` Available). Permanence still tracked by inbox [057](../057.md) (bake into `scripts/mock-cluster.sh`). Without it, CPU/memory reads are empty regardless of replica count.

**Fixture:** `bash scripts/dev-env.sh 5 up` → Hydra `bex-bootstrap` client-credentials → CP `POST /v1/tenants` + `PATCH …/billing-excluded` → `POST /v1/services` with Render-shaped body (`image.imagePath` + `image.ownerId:""`, `envVars: WHOAMI_PORT_NUMBER=3000`, `serviceDetails.numInstances:3`). Example ids: `tea-dak8489jg4r4nelqraug` / `srv-dak848pjg4r4nelqrb00` (earlier walk `tea-dak7oh1jg4r9je1e78n0` / `srv-dak7oh1jg4r9je1e78og`).

**Observed (REST):**

| Check (m89–m91) | Verb | Result |
| --- | --- | --- |
| Per-instance CPU | `GET /v1/metrics/cpu?resource=$SVC` | **3** series, distinct `instance` labels |
| Per-instance memory | `GET /v1/metrics/memory?resource=$SVC` | **3** series |
| Instance count | `GET /v1/metrics/instance-count?resource=$SVC` | value **3** |
| Percentages (m90) | `GET /v1/metrics/cpu?resource=$SVC&percentage=true` | **3** series, `unit=percentage` |
| Aggregation (m89) | `GET /v1/metrics/cpu?resource=$SVC&aggregateAllMethod=AVG` | **1** series |
| Instance selection (m89/m91) | `GET /v1/metrics/cpu?resource=$SVC&instance=$PICK` | **1** series for the selected id |

**Observed (GraphQL):** `metrics(query:{filters:[{field:RESOURCE,values:[$SVC]},{field:INSTANCE,values:[$PICK]}], name:CPU})` → **1** series; `percentage:true` + `aggregateAllMethod:AVG` → **1** series, `unit=percentage`.

**Cross-surface:** REST and GraphQL agree on series count / percentage unit / selected instance. MCP `POST /mcp` `tools/call get_metrics` returned a non-JSON body without a prior MCP session handshake — not treated as a metrics-core FAIL (shared `Service.Metrics` already covered by unit parity tests); no ADR018 claim change.

**Caveats (not FAILs):** mock cluster still lacks Traefik, so App `status.phase` stays Failed even with whoami listening on `:3000`; metrics-server still reports usage while pods exist. Dashboard Metrics tab was not screenshot'd this session — the card consumes the same GraphQL `metrics` query verified above. Scale 3→2 did not always drop live series before pods disappeared (ephemeral Failed Apps); selection + aggregation cover the m89 selector contract.

Evidence files: `/tmp/m98-t002-evidence.txt`, `/tmp/m98-t002-evidence2.txt`.

### t003 — BLOCKED (2026-09-14). Agent-turn series need the agent-session opt-in + model key.

Missing inputs (exact):

1. `touch .pm/w5/dev-5/.agent/enabled` then `bash scripts/dev-env.sh 5 agent-up` (wires `BEX_OPENSANDBOX_URL` / gateway; without it `bex_agent_session_*` series are not registered).
2. A real model API key in OpenBao for a repo-less turn (same class of credential `w3/m79` waits on — never print `.env`).

Control-plane `:56050/metrics` on a non-agent `dev-5` correctly shows **no** `bex_agent_session_*` vectors until a first observation. Annotated on `done/m88/README.md`.

### t005 — BLOCKED (2026-09-14). Agent verify legs need agent opt-in; leg 1b needs GitHub App.

Missing inputs (exact):

1. Same agent opt-in as t003 (`dev-5/.agent/enabled` + `agent-up` + healthy OpenSandbox).
2. For `scripts/agent-session-verify.sh` **leg 1b** only: a developer-supplied GitHub App installation on the workspace — identical blocker to `w3/m79` t012.
3. For m66's real `claude-code-acp` turn: model key (same as t003).

Repo-less legs and the ACP typed-parts check were not faked. Annotated on `done/m65/README.md` and `done/m66/README.md`.

### t006 — PASS (2026-09-14). No in-task code fixes; REST↔GraphQL parity recorded.

No product code changed in this sweep (verification + board only). Metrics four-surface note: REST + GraphQL live-agree; MCP handshake not re-driven (see t002). ADR018 Metrics rows unchanged.

### t007 — PASS (2026-09-14). No code to simplify.

### t008 — PASS (2026-09-14). No in-task FAIL fixes; existing metrics unit suites remain the regression net.

### t009 — PASS (2026-09-14). Source deferred lines annotated; milestone moved to `done/m98/`.
