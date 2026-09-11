# w5 · m95 — Sandbox provisioning SLI: first-party series, alert, and panel

**Worker:** worker5 **Goal:** sandbox creation outside an agent session (`POST /v1/sandboxes`) gets a first-party Prometheus signal, a paging rule with a traffic floor, and a panel — closing the one row of ADR088's tenant-facing coverage table that still reads "none (accepted for now), owner: unfiled". **Status:** todo (t001-t005 done; t006 closeout pending live verification)

## Tasks (in order)

| id   | title                                                                                                  | est | depends_on |
| ---- | ------------------------------------------------------------------------------------------------------ | --- | ---------- |
| [t001](done/t001.md) — **DONE** | Emit create/terminate outcome counters and a create-latency histogram from the sandbox service         | 45m | —          |
| [t002](done/t002.md) — **DONE** | `SandboxProvisionFailing` rule with a traffic floor, promtool fire/no-fire tests, scrape keep-list      | 30m | t001       |
| [t003](done/t003.md) — **DONE** | Panel beside the agent-session provisioning panels; coverage check green; ADR088 row flips to "alert"  | 30m | t002       |
| [t004](done/t004.md) — **DONE** | Simplify                                                                                               | 20m | t003       |
| [t005](done/t005.md) — **DONE** | Test coverage                                                                                          | 30m | t003       |
| t006 | Closeout                                                                                               | 10m | t005       |

## Definition of done

- `bex_sandbox_create_total{outcome}`, `bex_sandbox_create_seconds{outcome}` (histogram), and `bex_sandbox_terminate_total{outcome}` exist on the control-plane `/metrics` registry with a closed outcome set; authorization denials and bad requests are **not** counted as provisioning failures.
- A forced create failure on dev-5 (runtime unreachable or capacity exhausted) increments the `failed`/`capacity` outcome; `SandboxProvisionFailing` fires in the promtool fire test and stays silent below the traffic floor and at a low ratio.
- The Prometheus scrape keep-list admits the `bex_sandbox_.*` family; `scripts/obs-coverage-check.sh` reports the new alert covered by a panel with no new waiver; `bash scripts/gitops-validate.sh` passes.
- `docs/ADR088-platform-observability-ui.md` § tenant-facing surface coverage has no cell reading "Owner: unfiled"; the Sandboxes row names the rule and this milestone; the per-dashboard SLI table lists the new series.

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w5` 2026-09-10 proposal 3, from `docs/ADR088-platform-observability-ui.md:108` — "Sandboxes | — | — | **none (accepted for now)** … a `/v1/sandboxes`-specific regression is not [caught]. Owner: unfiled".
- **Facts verified 2026-09-10:** `lego/backend/internal/sandbox/{rest,service}.go` emit no Prometheus series; no `bex_sandbox_*` name exists in the backend or in `deploy/gitops/base/prometheus.yaml`; the sibling pattern is `lego/backend/internal/agentsessions/completion_metrics.go` (`bex_agent_session_provision_seconds`, registered on `metricRegistry` in `cmd/api/main.go:170`) with the `AgentSessionProvisionFailing` rule at `prometheus.yaml:~1215` and its promtool tests at `rules/alerts_test.yml:~1108-1139`.
- **Goal linkage:** pillar 5 (E2B-compatible sandboxes) is shipped and this is its only unwatched tenant surface; ADR088's "falsifiable green" principle; ADR008 § What's next, "keep SLIs honest".
- **Expected outcome:** a `/v1/sandboxes` regression (runtime down, capacity exhausted, image pull failing) pages within one evaluation window and is visible on the ops board next to the agent-session view, instead of being discovered by a tenant.
- **Why now:** the alert/panel/coverage-check pattern was built two days ago in `w3/m83`, so the marginal cost is lowest now; the coverage table explicitly leaves this row unowned, which the board's own rule says should not persist.
- **Render parity omitted:** no REST/GraphQL/MCP/dashboard contract changes — instrumentation, an alert rule, and an ops panel only.

## Evidence (2026-09-10)

**Scoping decision, and why it is not the obvious one.** `createResolved` is shared by the public `/v1/sandboxes` verb and the agent-session dispatch path, so instrumenting it would have been the shortest edit — and wrong. Agent sessions already report through `bex_agent_session_provision_seconds`, so counting them here would make one substrate incident fire two alerts while leaving the actual ADR088 gap ("is the sandbox API itself healthy") still unanswered. The observation therefore sits on `Create`, the public verb, and `TestAgentSessionCreatesStayOutOfTheSandboxSurfaceSignal` pins it: moving the instrumentation into the shared path makes it fail with the counter at 2.

**Refusals are counted but kept out of the ratio.** `rejected` (bad request, denied, billing gate) and `unavailable` (runtime not configured) are decided before any runtime call. Counting them in the alert's ratio would let a client spraying invalid requests either raise a false alarm or — more dangerously — dilute a real fault into silence. A promtool case proves the second: 100× refusals alongside two genuinely failed attempts still pages at 100%. `capacity` is deliberately on the failure side, because it means a pod never became ready in the platform's wait — a substrate symptom bex attributes to a plan limit, not a clean refusal. Refusals are also excluded from the latency histogram, where a burst of instant 400s would drag p95 down and mask a slow substrate.

**Verification.**

- `promtool` 2.55.1 (the pinned chart appVersion) installed locally, so the rules were actually executed rather than deferred to CI: `check rules` reports 50 rules, and the five new unit cases pass — fires on failures, fires on ready-timeouts, fires despite a refusal flood, stays silent at ~9% failures, stays silent under the traffic floor.
- `scripts/obs-coverage-check.sh` moved from **UNCOVERED SandboxProvisionFailing** to `covered … (via bex_sandbox_create_total)`; final state 48 covered, 1 waived, with the pre-existing waiver unchanged. The check earned its keep here — it failed the build the moment the alert landed without a panel.
- New guard in `scripts/gitops-validate.sh`: every alerted `bex_*` family must survive the bex-api scrape keep-list. This is the m86 death-layer lesson — a rule evaluating a never-scraped series reports health it cannot observe — and the keep-list is a single regex, so dropping a family is a one-character change with no other signal. Mutation-proven: removing `sandbox` from the regex fails the validator with a named message.
- Mutation checks: classifying refusals as failures fails the classifier test; moving instrumentation to the shared path fails the scoping test.
- `make lint` 0 issues across all four modules; `go test ./internal/sandbox/ ./internal/api/` green; `scripts/gitops-validate.sh` exit 0.

**Observed in passing, not fixed (out of scope).** The `bex-api-gateways` dashboard has a pre-existing panel overlap: "Agent terminal convergences (1h)" (id 5, y=27 x=0 w=8 h=9) sits inside "Webhook deliveries capped (15m)" (id 8, y=27 x=0 w=24 h=8). Grafana reflows overlapping panels on load, so it is cosmetic, and the geometry test that would catch it covers only the CLI usage board. Worth a follow-up note rather than an unrelated edit inside this milestone.

**Not verified live.** `bex_sandbox_create_total` appearing on the production scrape and the two panels rendering at obs.bex.co are owed after deploy (t006). The series reaching Prometheus at all is what the new keep-list guard protects statically.
