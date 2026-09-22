# w9 · m166 — Reject invalid service patches before applying any settings

**Worker:** worker9 **Goal:** a rejected multi-field service update leaves the service unchanged and does not trigger a deployment. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Validate and authorize the whole service patch before its first write | 60m | — |
| t002 | Verify the shared patch and setter blast radius | 45m | t001 |
| t003 | Render parity across CLI, REST, MCP, GraphQL, and dashboard | 25m | t002 |
| t004 | Simplify | 15m | t003 |
| t005 | Test coverage and live CLI regression | 40m | t003, t004 |
| t006 | Closeout | 10m | t005 |

## Definition of done

- On a disposable free web service, `bex services update "$SERVICE_ID" --max-shutdown-delay 8 --health-check-path not-a-path --confirm -o json` exits nonzero with the existing Render-shaped HTTP 400; the old shutdown delay and health path remain unchanged, no deploy is created, and no setting-applied audit event is emitted. The same holds for a valid name plus invalid health path.
- Validation, service-kind, plan-policy, protected-environment, and authorization refusals are resolved before any field in that request is persisted. Validation uses the proposed combined state, retaining valid maintenance-disable/free-downgrade and source/credential combinations.
- A valid multi-field update applies every requested setting, returns the existing success shape, records the expected single config-change deployment when appropriate, and reaches a working runtime. Single-field and empty-patch behavior remain covered.
- REST and MCP share this behavior. GraphQL/dashboard single-setting callers retain their existing policy and error contracts. A list of the affected callers and service kinds, plus meaningful regression results, accompanies the change.
- Live verification uses newly owned fixtures with the pinned CLI; deletion is verified by detail/list absence and runtime teardown. All tasks are complete before moving this milestone to `done/`.

## Source + Goal linkage

- **Source:** user-requested looping `/qa-find-bugs-cli`, filed in w9; production sweep 1 on 2026-09-22 UTC using the authorized QA user's human device-login session. Complete sanitized reproduction, wire exchange, and diagnosis are in [t001](t001.md).
- **Goal linkage:** [ADR008](../../../docs/ADR008-vision.md) requires dependable hosting operations for people and agents; [ADR006](../../../docs/ADR006-bex-api.md) puts these semantics in one shared core behind thin adapters. A command reporting rejection while changing live configuration makes safe automation impossible.
- **Expected outcome:** invalid compound updates cannot rename a service or roll its running workload before returning an error.
- **Why now:** the installed customer CLI reproduced both effects on production. A rejected shutdown-delay change created a deployment that reached `live`; this is observable behavior, not a hypothetical ordering concern. The shared table has 22 entries and two production adapters, so repairing only the demonstrated field pair would leave the same failure class elsewhere.
- **Parity:** included because this changes tenant-facing mutation semantics. The local v2.27.0 upstream command/request/response code is the tested CLI oracle. [RFC 5789 §2](https://www.rfc-editor.org/rfc/rfc5789.html#section-2) requires a PATCH document to be applied atomically; live Render behavior has not been tested and must not be claimed.

## Scope and related work

This milestone addresses deterministic validation/policy/authorization rejection **after earlier fields have already succeeded**. It must not promise that prevalidation alone solves an unexpected database/Kubernetes failure midway through persistence; document that separate boundary accurately and preserve truthful errors.

- [w1/done/m78](../../w1/done/m78/README.md) unified the ordered patch table without changing semantics; it did not establish mutation atomicity.
- [w6/done/m51](../../w6/done/m51/README.md) coalesced configuration changes into one deploy row. Its deferred batch flush deliberately records already-applied fields even after a later failure; suppressing that row would hide this bug, not fix it.
- [w4/123](../../w4/123.md) covers disk guard failures between row and CR writes; [w4/m130](../../w4/m130/README.md) covers an env-revision apply/409 conflict. Neither covers this accepted-setting/invalid-setting pair.
- [w7/m152](../../w7/m152/README.md) covers project/environment partial mutations. `core.PatchOps` has sibling consumers there; avoid turning this fix into an unverified generic rollback mechanism.

## Live cleanup

Owned fixtures `srv-dap12h94dm7c7390q720` and `srv-dap13i3bdpcs73f5ed3g` were deleted by recorded ID on 2026-09-22 at 05:35 UTC. Both detail endpoints and former runtime URLs returned 404 at 05:36 UTC; the workspace list contained only its original `hello-go` service. The source-backed fixture's queued build was canceled first. No baseline resource was mutated.
