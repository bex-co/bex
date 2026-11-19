# w5 · m501 — Production diagnostics stop printing tenant data into public Actions logs

**Worker:** worker5 **Goal:** the CI path an operator uses during an incident keeps working, and no production job publishes which tenants run what on bex: tenant identifiers, node addresses and tenant event text stay out of this public repository's Actions logs. **Status:** in progress (t001–t008 done; t009 closeout checks the live runs)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Inventory every job that holds production credentials and what it prints — **DONE** | 45m | — |
| [t002](done/t002.md) | `scripts/cluster-diagnose.sh`: full detail locally, tenant identifiers hashed with `--public-log`; Cluster diagnose calls it — **DONE** | 1h30m | t001 |
| [t003](done/t003.md) | The other production jobs t001 finds print through the same redaction — **DONE** | 45m | t002 |
| [t004](done/t004.md) | `github-actions-validate.sh` refuses raw `kubectl` output in a job that holds production credentials — **DONE** | 1h | t002 |
| [t005](done/t005.md) | Delete the earlier Cluster diagnose run logs and any other run t001 finds carrying tenant data — **DONE** | 15m | t001 |
| [t006](done/t006.md) | ADR019 states what a production job may print in this public repository — **DONE** | 30m | t004 |
| [t007](done/t007.md) | Simplify — **DONE** | 15m | t003, t004, t005, t006 |
| [t008](done/t008.md) | Test coverage — **DONE** | 30m | t007 |
| [t009](t009.md) | Closeout | 15m | t008 |

Total: **9 tasks, about 5h45m**.

## Definition of done

1. A Cluster diagnose run on `main` for a real service id prints, in its public log, no tenant namespace or workspace id, no service id or name, no tenant pod name, no node name or IP and no tenant event text. It still reports pending and failing pods by count and reason, node readiness and allocation, and autoscaler state.
2. `bash scripts/cluster-diagnose.sh <srv-id>` run locally with the production kubeconfig (`scripts/fetch-app-kubeconfig.sh`) prints the full, unredacted detail.
3. `bash scripts/github-actions-validate.sh` fails a fixture workflow whose production-credentialed job runs `kubectl get pods -A` straight to the log, and passes the repository's workflows.
4. The 8 earlier Cluster diagnose runs answer 404 (`gh run view <id>`), and every other run t001 finds carrying tenant data is deleted.
5. ADR019 states what a production job may print in this public repository, and the redaction and validator are covered by tests.

## Findings

**t001 inventory (2026-11-18).** Every job that holds production credentials (a `production-*` environment or the admin.conf fetch), what its log printed, and what happened to it. The scan read every run log still on GitHub (848 runs) and counted bex ids not already published in the repository outside `.pm/`, node names, node addresses, probe failure text, namespace lists and service-name JSON.

| Job | What it printed | Tenant data | Fix | Earlier runs |
| --- | --- | --- | --- | --- |
| `cluster-diagnose.yml` | raw pods `-A -o wide`, `describe pod`, nodes `-o wide`, `describe nodes`, Machines, autoscaler status and logs | yes: workspace and service names, pod names, node names and addresses, tenant events | `scripts/cluster-diagnose.sh --public-log` (t002), service id read from the event payload and masked | 8 runs deleted (404), with 44033022163 |
| `ssh-edge-liveness.yml` `request-logs-liveness` | every tenant namespace with request or app logs in 6 h | yes: which workspaces are active | prints the count only; a FAIL list goes through `public_log.py` | 279 runs' logs deleted |
| `app-cluster.yml` | `kubectl wait node --all`, `get nodes -o wide` | node names and both addresses | wait silenced, node table through `public_log.py` | 35 runs' logs deleted |
| `platform-rebuild.yml` | the full rebuild JSON (`tee`) | every eligible service's workspace, name and commit | counts, then namespace hash and runtime per item | 2 runs' logs deleted |
| `hcloud-csi-rollout.yml` inventory | every service with a disk (namespace, App, service id, PVC) | yes | inventory through `public_log.py` | 14 runs' logs deleted |
| `sandbox-measure.yml` | MEASURE lines with node names and addresses, other workspaces' sandbox namespaces | yes | measure step through `public_log.py` | 18 runs' logs deleted |
| `disk-snapshot-drill.yml` | the dispatched App's pods, node, PVC, events (probe failure text) | yes | `--public-log` (no event or condition messages), inputs masked, output through `public_log.py` | 12 runs' logs deleted |
| `container-policy-drill.yml` | the dispatched namespace's Apps with image and repository | yes | `--public-log` (no image or repository), inputs masked, output through `public_log.py` | 6 runs' logs deleted |
| `isolation-matrix.yml` | its own disposable workspaces; one run a node name in a fixture event; leftover-fixture audit greps cluster-wide lists | one run | audit greps through `public_log.py` | 1 run's log deleted |
| `keyvalue-restore-drill.yml` | the dispatched source KeyValue id and namespace in step `env:` headers | first-party fixture only (already in `docs/`) | inputs masked | none |
| `deploy.yml` `deploy` | control-plane address; on an openfga failure `get pods -o wide`, `describe pods` events | node names on failure only | `-o wide` dropped, events through `public_log.py` | none (no run hit) |
| every admin.conf fetch (deploy, alerting-heartbeat, openbao/disk/keyvalue drills, ...) | `app kubeconfig -> ... (via CP <address>)` | control-plane endpoint address only | `fetch-app-kubeconfig.sh` masks each discovered address under GitHub Actions | not deleted: the API endpoint address, not tenant data |
| `deploy-canary.yml`, `ssh-edge-liveness.yml` tenant-view, `cli-release.yml`, `infra.yml`, `production-ssh-trust.yml`, `snapshot.yml`, `alerting-heartbeat.yml`, `openbao-restore-drill.yml`, `helm-mirror.yml`, `mobile-release.yml` | canary fixtures, agent session branches, Terraform subnet and load-balancer address, control-plane host keys, Packer builder address | no (platform facts or bex's own fixtures) | none needed | none |

**Deviation (t005):** apart from the Cluster diagnose runs, which DoD 4 names and which answer 404, the runs above had their **logs** deleted (`DELETE /actions/runs/{id}/logs`; each answers 404 now) rather than the runs. The tenant data was in the logs only. `ssh-edge-liveness` is a scheduled monitor, and `scripts/ci-red-streak.sh` reads its run history, so deleting 279 green runs would have left a red streak with nothing to compare against. No `psql` or `curl` output in a production job printed tenant data, so t004 has no follow-up.

**t007 Simplify (2026-11-19):** three reviews (reuse, quality, efficiency) over the diff. Fixed:

- check 11 moved out of the validator's heredoc into `scripts/lib/public_log_check.py`, with one kubectl argument parser. The earlier two disagreed, so `-n bex-system get -o name nodes` passed.
- `2>&1 |` no longer splits a pipeline, `>&2` no longer counts as a redirect, and a review comment carries across a run of comment lines.
- a platform component's `logs` are no longer exempt: `deploy.yml` now pipes openfga's through the redactor.
- one namespace set and tag pattern, shared from `public_log.py`.
- diagnose stops with one line when the API server does not answer, instead of a traceback after about 2 minutes. Its kubectl errors never raise, Machines and nodes are fetched once, and the autoscaler is found by label, not by listing every pod.
- the redactor skips the node lookups when the cluster does not answer, and the workflow step goes through it once more.
- autoscaler lines keep `0/3 nodes`.

Not changed: the two kubectl wrappers have different contracts (silent text vs. JSON or an error line), and checks 10 and 11 share only their job splitter.

**t008 Test coverage:** each definition-of-done item and the test that would catch its regression.

1. `scripts/cluster-diagnose.test.sh` (stub kubectl, synthetic fixtures): the public view carries no tenant id, name, node name or address, image or event text, and still reports counts, reasons, readiness, allocation, Machines and the autoscaler. Eight mutations of the redaction (an event message, `default` as a platform namespace, the live node list, autoscaler pod references, a tenant pod named autoscaler, a fail-open name, the unreachable stop, `0/3` garbling) each turn it red. The live check is the run on `main` (t009).
2. The same test's local-run case, plus a local run against production on 2026-11-19 that printed the full detail.
3. `scripts/github-actions-validate.test.sh` check 11: 18 fixtures, the `kubectl get pods -A` one among them, and the validator passes the real tree. Run over the pre-milestone workflows, it flags all 21 printers.
4. No repository test can hold GitHub state. `gh api repos/bex-co/bex/actions/runs/<id>` answers 404 for the 9 Cluster diagnose runs, and `.../logs` answers 404 for the 367 other runs.
5. ADR019 §Decision 5 states the rule, and the tests above cover it, along with `scripts/fetch-app-kubeconfig.test.sh` (control-plane addresses masked in CI only).

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w5`, round 10 item 1 (2026-11-18), selected with `/pmx them all to w5`.
- **Evidence:** run 44033022163 (2026-11-17, dispatched with a placeholder service id to read production) printed a tenant workspace id, one of its service names inside a build pod's name, every node's name with its internal and public IP, the Machine objects and the cluster autoscaler's status. It was deleted on 2026-11-18. Cluster diagnose has 9 runs since 2026-10-24 (`gh run list --workflow cluster-diagnose.yml`), and the repository is public (`gh repo view --json visibility`), so their logs are readable by anyone. `.github/workflows/cluster-diagnose.yml` prints 12 `kubectl` reads; `app-cluster.yml` prints 2. Nothing in `docs/` or `scripts/github-actions-validate.sh` covers what a public log may contain.
- **Not exposed:** tenant environment values. They reach pods through `spec.envFromSecret` ([ADR006 § Env vars](../../../docs/ADR006-bex-api.md)), which `kubectl describe pod` names by Secret only.
- **Goal linkage:** tenant isolation ([ADR043](../../../docs/ADR043-tenant-namespace-isolation.md)), infrastructure credentials and CI posture ([ADR019](../../../docs/ADR019-infra-credentials.md) §Decision 5), the security-review lineage.
- **Expected outcome:** the incident tool stays usable from CI, and tenant detail is one local command away (`~/.ssh/bex` and `HCLOUD_TOKEN` reach production from an operator's checkout), never in a public log.
- **Why now:** this workflow is the documented incident tool, so every incident adds another public log, and local production access now works, so the CI copy no longer needs full detail.
- **Render parity:** omitted. This is internal tooling with no tenant-facing surface.
