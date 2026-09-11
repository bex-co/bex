# w5 · m98 — Live-verification sweep on dev-5

**Worker:** worker5 **Goal:** the five residuals that closed w5 milestones recorded as "deferred — cluster unavailable" (and that nothing on the board owns) each get a dated PASS or FAIL line from a live run, with every FAIL fixed in-task when small or filed as an inbox note carrying `file:line`. **Status:** todo (t001, t004 done; t002 blocked — no metrics-server or Prometheus on any dev-N stack; t003/t005 blocked on the agent-session opt-in. See § Evidence)

## Tasks (in order)

| id   | title                                                                                                      | est | depends_on             |
| ---- | ---------------------------------------------------------------------------------------------------------- | --- | ---------------------- |
| [t001](done/t001.md) — **DONE** | Bring up `dev-5`, reprovisioning the shared kind/CAPD cluster if needed; record what was rebuilt            | 30m | —                      |
| t002 | m89–m91: multi-replica Metrics walk — instance selection, aggregation, percentages, selection persistence    | 45m | t001                   |
| t003 | m88: agent-turn latency/outcome series via a repo-less session, or record the model-key blocker             | 45m | t001                   |
| [t004](done/t004.md) — **DONE** | 053: confirm the production availability board's edge 5xx log panel shows platform-host request lines       | 15m | —                      |
| t005 | m65 / m66: `agent-session-verify.sh` legs incl. 1b and a real ACP turn, or record the GitHub-App blocker    | 30m | t001                   |
| t006 | Render parity                                                                                              | 30m | t002, t003, t004, t005 |
| t007 | Simplify                                                                                                   | 20m | t006                   |
| t008 | Test coverage                                                                                              | 30m | t006                   |
| t009 | Closeout                                                                                                   | 15m | t008                   |

## Definition of done

- Every residual named in t002–t005 has a dated **PASS** or **FAIL** line in this README's Evidence section with the exact command/verb, the surface exercised, and the observed result; "reasoned from code shape" is not evidence. A residual that is blocked by credentials this environment cannot supply gets a **BLOCKED** line naming the exact missing input (same standard as `w3/m79` t012).
- Every **FAIL** is fixed inside its task (≤ ~30m, with a regression test) or filed as a `w5/NNN` inbox note carrying `file:line`, and the source milestone's deferred bullet is annotated with the outcome and pointer.
- No w5 closeout under `done/` still reads "deferred", "not verified live", or "awaiting live verification" without an annotation pointing here.

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w5` 2026-09-10 proposal 6 — a sweep of w5 closeouts: `done/m89/README.md:59` ("Live multi-replica walk deferred when cluster unavailable"), `done/m88/README.md:58` ("live dev-5 registry/panel walk deferred (kubectl/dev-5 unavailable at closeout)"), `done/m65/README.md:58` ("`scripts/agent-session-verify.sh` legs — including the new 1b — need a real cluster + GitHub App and have not been run"), `done/m66/README.md:26` ("Live verification (a real `claude-code-acp` turn)…"), and `done/053.md` ("awaiting live verification by the coordinator"). None of these is among the ten residuals `w1/m139` owns.
- **Facts verified 2026-09-10:** the local kind cluster's API refused connections at 21:06 local (`infra/local/bex.kubeconfig`, `127.0.0.1:55003`); `w1/m139`'s evidence records the control plane crash-looping on etcd latency on 09-09 and a green `mock-cluster.sh` re-run on 09-10, so bring-up may again need reprovisioning (pre-approved for `dev-N` and `mock-cluster.sh` per root `CLAUDE.md`).
- **Goal linkage:** pillar 1 correctness of already-shipped work (Metrics page parity, agent-session reliability, ADR088 incident explainability); the board's own rule that a milestone lands in `done/` when its observable end state is real.
- **Expected outcome:** the deferred-verification debt in w5 stops compounding; any defect hiding behind "identical code shape" surfaces with evidence; the 053 panel is proven to explain platform-API incidents.
- **Why now:** each new closeout since 2026-08-20 has added to this list, and `w1/m139` is doing the same sweep for its own residuals — running both while the cluster is being nursed back shares the bring-up cost. Honest caveat: the cluster is down at filing time, so this ranks below the shipping milestones (m94–m97).
- **Render parity included:** in-task fixes may touch the dashboard Metrics UI or the metrics/events API; t002 explicitly compares the dashboard read against REST/GraphQL/MCP for the same window.

## Evidence (2026-09-11)

### t001 — PASS. Cluster and `dev-5` rebuilt from nothing.

The shared kind/CAPD cluster did not exist at all when this milestone started (API refusing on `127.0.0.1:55003`, no `bex-*` node containers). `scripts/mock-cluster.sh` brought it back: three nodes Ready (`bex-8ng2b-x4qh9` control-plane, `bex-tenant-0-…`, `bex-worker-0-…`), reusing the existing 12-day-old node containers rather than reprovisioning them. `scripts/dev-env.sh 5 up` then came up clean — Kratos, Kratos admin, Hydra admin, Hydra public and bex-api all answering 200, and the verification inventory **ALL GREEN** (default StorageClass, CNPG operator, Loki, bex-api). `bex-api` was rebuilt from the current tree at 00:27. No other workstream's stack was touched; `dev-8` namespaces were observed running concurrently and left alone.

### t004 — PASS. The platform edge-host log retention is real in production.

Inbox note `053` extended the log-shipper allowlist so the availability board's 5xx log panel can explain platform-API incidents, and closed with "awaiting live verification by the coordinator". Verified against production Loki (port-forward, `{type="request"}` over 24h): **all five** hosts return retained request lines — `api.bex.co`, `dashboard.bex.co`, `oauth.bex.co`, `auth.bex.co`, `obs.bex.co`. The panel now has data to show for exactly the incidents it exists for.

### t002 — BLOCKED, structurally, and the first-stated reason was wrong.

The m89/m90/m91 residuals cannot be discharged on any `dev-N` stack as currently built. Worth stating precisely, because the obvious explanation is the wrong one:

- The Metrics page's CPU/memory/instance series come from **metrics-server** (`internal/metrics/source.go:44`, `metrics.k8s.io`), not Prometheus. The local cluster has no metrics-server: `v1beta1.metrics.k8s.io` is **NotFound**.
- Prometheus is missing too, which independently blocks the request-metrics half: `main.go:521` wires it "only when `BEX_PROM_URL` is set", and `scripts/dev-env.sh` never sets that variable. The cluster's `monitoring` namespace holds only Loki and log-shipper.

So this is not "the cluster was down" — it is that a `dev-N` stack has no resource-metrics source at all. **Enabling work, not a retry:** add metrics-server to `scripts/mock-cluster.sh` (CPU/memory/instances) and, if the request-metrics half is also wanted, a Prometheus plus `BEX_PROM_URL` in `dev-env.sh`. Until then these three residuals are only dischargeable against production.

### t003 and t005 — BLOCKED on the agent-session opt-in, not on the cluster.

The agent stack is opt-in per environment: `agent_enabled()` is `[ -f "$AGENTDIR/enabled" ]` (`scripts/dev-env.sh:425`), and this bring-up did not enable it, so bex-api started without `BEX_OPENSANDBOX_URL`/gateway wiring and the OpenSandbox port-forward (`:61050`) is not listening. The in-cluster OpenSandbox control plane itself **is** running, though not healthily: `opensandbox-controller-manager` and `opensandbox-server` show **63 and 55 restarts** over three days, which is worth a look before either task is retried.

Clearing them needs, in order: touch the enable marker and re-run `dev-env.sh 5 up`; a `bex-agent-sandbox:dev` image; a real model key for t003's turn; and, for t005's leg 1b only, a developer-supplied GitHub App installation — the same input `w3/m79` t012 has been waiting on.

### Incidental verification worth keeping

The dev-5 control-plane registry (`:56050`) serves `bex_api_http_*`, `bex_billing_*` and `bex_push_*`, and shows **no** `bex_agent_session_*` or `bex_sandbox_*` series. That is correct rather than a defect: both are labelled Prometheus vectors, which emit nothing until a first observation, and `srv.Sandbox` is nil without `BEX_OPENSANDBOX_URL`, so m95's metrics are deliberately not registered when the sandbox surface is absent. It does mean m95's series cannot be observed on a `dev-N` stack without the agent opt-in either.
