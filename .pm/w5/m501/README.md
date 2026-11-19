# w5 · m501 — Production diagnostics stop printing tenant data into public Actions logs

**Worker:** worker5 **Goal:** the CI path an operator uses during an incident keeps working, and no production job publishes which tenants run what on bex: tenant identifiers, node addresses and tenant event text stay out of this public repository's Actions logs. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](t001.md) | Inventory every job that holds production credentials and what it prints | 45m | — |
| [t002](t002.md) | `scripts/cluster-diagnose.sh`: full detail locally, tenant identifiers hashed with `--public-log`; Cluster diagnose calls it | 1h30m | t001 |
| [t003](t003.md) | The other production jobs t001 finds print through the same redaction | 45m | t002 |
| [t004](t004.md) | `github-actions-validate.sh` refuses raw `kubectl` output in a job that holds production credentials | 1h | t002 |
| [t005](t005.md) | Delete the earlier Cluster diagnose run logs and any other run t001 finds carrying tenant data | 15m | t001 |
| [t006](t006.md) | ADR019 states what a production job may print in this public repository | 30m | t004 |
| [t007](t007.md) | Simplify | 15m | t003, t004, t005, t006 |
| [t008](t008.md) | Test coverage | 30m | t007 |
| [t009](t009.md) | Closeout | 15m | t008 |

Total: **9 tasks, about 5h45m**.

## Definition of done

1. A Cluster diagnose run on `main` for a real service id prints, in its public log, no tenant namespace or workspace id, no service id or name, no tenant pod name, no node name or IP and no tenant event text. It still reports pending and failing pods by count and reason, node readiness and allocation, and autoscaler state.
2. `bash scripts/cluster-diagnose.sh <srv-id>` run locally with the production kubeconfig (`scripts/fetch-app-kubeconfig.sh`) prints the full, unredacted detail.
3. `bash scripts/github-actions-validate.sh` fails a fixture workflow whose production-credentialed job runs `kubectl get pods -A` straight to the log, and passes the repository's workflows.
4. The 8 earlier Cluster diagnose runs answer 404 (`gh run view <id>`), and every other run t001 finds carrying tenant data is deleted.
5. ADR019 states what a production job may print in this public repository, and the redaction and validator are covered by tests.

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w5`, round 10 item 1 (2026-11-18), selected with `/pmx them all to w5`.
- **Evidence:** run 44033022163 (2026-11-17, dispatched with a placeholder service id to read production) printed a tenant workspace id, one of its service names inside a build pod's name, every node's name with its internal and public IP, the Machine objects and the cluster autoscaler's status. It was deleted on 2026-11-18. Cluster diagnose has 9 runs since 2026-10-24 (`gh run list --workflow cluster-diagnose.yml`), and the repository is public (`gh repo view --json visibility`), so their logs are readable by anyone. `.github/workflows/cluster-diagnose.yml` prints 12 `kubectl` reads; `app-cluster.yml` prints 2. Nothing in `docs/` or `scripts/github-actions-validate.sh` covers what a public log may contain.
- **Not exposed:** tenant environment values. They reach pods through `spec.envFromSecret` ([ADR006 § Env vars](../../../docs/ADR006-bex-api.md)), which `kubectl describe pod` names by Secret only.
- **Goal linkage:** tenant isolation ([ADR043](../../../docs/ADR043-tenant-namespace-isolation.md)), infrastructure credentials and CI posture ([ADR019](../../../docs/ADR019-infra-credentials.md) §Decision 5), the security-review lineage.
- **Expected outcome:** the incident tool stays usable from CI, and tenant detail is one local command away (`~/.ssh/bex` and `HCLOUD_TOKEN` reach production from an operator's checkout), never in a public log.
- **Why now:** this workflow is the documented incident tool, so every incident adds another public log, and local production access now works, so the CI copy no longer needs full detail.
- **Render parity:** omitted. This is internal tooling with no tenant-facing surface.
