# w5 · m95 — Sandbox provisioning SLI: first-party series, alert, and panel

**Worker:** worker5 **Goal:** sandbox creation outside an agent session (`POST /v1/sandboxes`) gets a first-party Prometheus signal, a paging rule with a traffic floor, and a panel — closing the one row of ADR088's tenant-facing coverage table that still reads "none (accepted for now), owner: unfiled". **Status:** todo

## Tasks (in order)

| id   | title                                                                                                  | est | depends_on |
| ---- | ------------------------------------------------------------------------------------------------------ | --- | ---------- |
| t001 | Emit create/terminate outcome counters and a create-latency histogram from the sandbox service         | 45m | —          |
| t002 | `SandboxProvisionFailing` rule with a traffic floor, promtool fire/no-fire tests, scrape keep-list      | 30m | t001       |
| t003 | Panel beside the agent-session provisioning panels; coverage check green; ADR088 row flips to "alert"  | 30m | t002       |
| t004 | Simplify                                                                                               | 20m | t003       |
| t005 | Test coverage                                                                                          | 30m | t003       |
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
