# w7 · m148 — Reliable local cluster bring-up

**Worker:** worker7 **Goal:** Publish a usable local substrate without concurrent provisioning or misleading success. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Serialize local cluster mutations | 40m | — |
| t002 | Publish validated kubeconfig atomically | 50m | w7/m148/t001 |
| t003 | Fail clearly on unmet substrate prerequisites | 45m | w7/m148/t002 |
| t004 | Verify the installed substrate before reporting success | 45m | w7/m148/t003 |
| t005 | Simplify | 25m | w7/m148/t001, w7/m148/t002, w7/m148/t003, w7/m148/t004 |
| t006 | Test coverage | 45m | w7/m148/t001, w7/m148/t002, w7/m148/t003, w7/m148/t004 |
| t007 | Closeout | 15m | w7/m148/t005, w7/m148/t006 |

## Definition of done

- [ ] A second invocation performs no provisioning or scale mutation while the first owns the lock.
- [ ] Normal exit and interruption release only the invoking process's lock; stale ownership has a safe recovery path.
- [ ] Failure before publication leaves the prior shared kubeconfig byte-identical.
- [ ] Published configuration reaches the intended local cluster with TLS verification enabled.
- [ ] Concurrent refresh cannot publish partial or stale configuration; credential contents are never printed.
- [ ] Required failures exit nonzero and prevent dependent stages from running.
- [ ] Healthy bootstrap still reaches CNI installation and later readiness checks in the correct order.
- [ ] No recovery touches production or silently destroys another workstream's resources.
- [ ] The script never prints its successful up message after a required check fails.
- [ ] A healthy local run and controlled failed prerequisite have recorded results.
- [ ] Report any local environment rebuilt; do not claim the script installs the bex operator or App CRD if that remains a separate workflow.

## Source + Goal linkage

- **Source:** Approved w7 brainstorm item 1 via `$pm all for w7`; `.pm/w9/blocked/060.md`, `.pm/w10/003.md`, and current `scripts/mock-cluster.sh` / `scripts/dev-env.sh`.
- **Goal linkage:** ADR008 hosting reliability: make local acceptance evidence dependable.
- **Expected outcome:** Concurrent bring-up cannot interleave mutations; failed generation preserves the prior kubeconfig; success means required substrate checks passed.
- **Why now:** Local cluster failures block verification in several queues. w1/043 installed missing components and w1/m72 unified dev harnesses; neither closed publication, concurrency, and failure-reporting gaps.
- **Render parity:** Omitted: local infrastructure and harness behavior only; no REST/GraphQL/MCP/UI contract change.
- **Sizing:** 180m implementation; 265m including standing closing tasks, 7 tasks.
