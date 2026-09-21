# w4 · m126 — A protected environment blocks suspend and delete but lets anyone swap the image a service runs, with no confirmation and no way to give one

**Worker:** worker4 **Goal:** the protected-environment guard covers the verbs that redefine or roll a running service, not just the two that take it away — and every guarded verb accepts the confirmation phrase on REST, GraphQL and MCP so a caller who means it can proceed. **Status:** todo

## Tasks (in order)

| id   | title                                                                             | est | depends_on |
| ---- | ----------------------------------------------------------------------------------- | --- | ------------ |
| t001 | Decide the guarded verb set and write the rule down in ADR032                        | 40m | —          |
| t002 | Guard source repointing (`setImage` / `setRepo` / registry credential)               | 45m | t001       |
| t003 | Apply the decision to the release-rolling settings verbs                             | 45m | t001       |
| t004 | Dashboard: surface the confirmation for the newly guarded verbs                      | 30m | t002, t003 |
| t005 | Render parity check (confirm argument + error shape across REST/GraphQL/MCP + UI)    | 30m | t002, t003, t004 |
| t006 | Simplify (`/simplify` over the changed code)                                         | 30m | t005       |
| t007 | Test coverage                                                                        | 45m | t005       |
| t008 | Closeout                                                                             | 15m | t007       |

## Definition of done

Every bullet is a probe run live during the hunt below, re-run against the fix with a `qa-` service in a `protected` environment:

- **Swapping the image is guarded.** `setImage(id:…, image:"…:35")` on a member of a `protectedStatus: protected` environment is refused with the named protected-environment error and the exact `sudo <verb> service <name>` phrase — the same shape `suspendService` already returns. Today it returns success and the service ends up running the new image.
- **The verb can accept a confirmation.** `setImage` (and every other newly guarded verb) takes a `confirm` argument on REST, GraphQL and MCP, and passing the phrase proceeds. Today `setImage` has no `confirm` argument at all, so the guard is not merely absent — it is unreachable.
- **The decision is written down.** ADR032's `protectedStatus` bullet states the complete guarded set and the rule that generates it, so the next verb added to the API has an answer rather than an omission. Verbs deliberately left out (Restart, Resume, webhook auto-deploy) keep their stated reasons.
- **Unprotected services are untouched.** The same calls against a service in an unprotected environment, or in no environment, succeed with no confirmation and no new argument required.
- **The guard's existing members still hold.** `deleteService` and `suspendService` on a protected member still refuse without the phrase and still succeed with it — verified live in this hunt and not to be regressed.
- **Tests fail without the fix.** A test asserting `setImage` on a protected member is refused fails against the pre-fix code.

## Source + Goal linkage

- **Source:** live `/qa-find-bugs` 2026-09-21 pass 141 (w4-targeted, `muse.env` credentials), journey 1. A `qa-` private service was put in a freshly created `protected` environment and every reachable mutation was run against it. Full matrix in [t001](t001.md). The headline: `suspendService` was refused with `"qa-20260921-pe" is a member of a protected environment; retry with confirm="sudo suspend service qa-20260921-pe" to suspend it`, and in the same session `setImage(image:"mendhak/http-https-echo:35")` was **accepted** — the service went from `mendhak/http-https-echo:latest` to `:35`, minted a deploy, and settled `Running` at `rev-6` on the new image. `setEnvVar`, `setEnvVars`, `scaleService`, `setIdleTimeout` were accepted too, producing four rollouts on a "protected" service without one confirmation. All fixtures (service, environment, project) deleted; the protected `deleteService` was refused without the phrase and accepted with it, which is the control proving the guard was armed throughout.
- **Goal linkage:** tenant safety on the environments surface, and the honesty of a control the product sells as protection — [ADR032](../../../docs/ADR032-environments.md) § `protectedStatus`, [ADR018](../../../docs/ADR018-render-parity.md) Environments row. The lineage is established: `w6/m19` built the guard for services, `w6/m37` extended it to Postgres and Key Value after m19's own closeout flagged the gap it had left. This is the same kind of extension, one axis over — from *which resources* are covered to *which verbs* are.
- **Expected outcome:** "protected" stops meaning "you cannot switch it off, but you can replace what it runs". A user who marks an environment protected gets a consistent answer from every verb that changes what is running there.
- **Why now:** the argument comes from the ADR's own stated principle rather than an opinion. ADR032 justifies guarding `Suspend` but not `Resume` as *"a protected environment blocks taking availability away, not restoring it"*, and guards the stack-apply override because it redefines an existing service. `setImage` redefines an existing service and rolls it out — the second rationale exactly — through a door the guard never learned about. The code already agrees the verb is consequential: `service.go:3562-3564` gates it on `can_create` because *"repointing a service at a new repo or image chooses the executable the operator runs with the service identity — create-like"*. The elevated authorization is there; the confirmation is not.
- **Render parity task included** because it adds a `confirm` argument and a coded error to service-mutation verbs on REST, GraphQL and MCP, and the dashboard must retry with the phrase.
- **DO_NOT_DO constraints honored:** no anti-goal touched. No new deletion path; the milestone only adds refusals and an opt-in confirmation.
