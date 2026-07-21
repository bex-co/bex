# w5 · m49 — Official Render CLI service CRUD: nested image ownership + deletion convergence

**Worker:** worker5 **Goal:** Make the unmodified official Render CLI's image-backed service create/update path conform to Render's OpenAPI contract, and restore residue-free, observably bounded service deletion when a repo-backed service is deleted during its first build. **Status:** todo (t001–t002 done)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Pin the official CLI's exact image create/update wire contract and reproduce the adapter drift — **DONE** | 30m | — |
| t002 | Accept and authorize nested `image.ownerId` on service create and patch — **DONE** | 45m | t001 |
| t003 | Diagnose the production deletion stall and safely retire its exact fixture | 45m | t002 |
| t004 | Restore finalizer convergence for delete-during-first-build | 60m | t003 |
| t005 | Extend the service CRUD verifier and pass production CLI acceptance | 60m | t004 |
| t006 | Render parity: reconcile image and deletion semantics across REST/GraphQL/MCP/UI | 30m | t005 |
| t007 | Simplify the REST image model, cleanup state machine, and verifier | 30m | t006 |
| t008 | Complete exact-payload, finalizer-failure, and verifier regression coverage | 45m | t006 |
| t009 | Closeout | 15m | t007, t008 |

## Definition of done

Against the deployed production API, the unmodified official Render CLI v2.21.0 can create and update an image-backed service using its exact nested `image.ownerId` payload, then read and delete it without a bex-specific workaround. A disposable repo-backed service deleted while its first build is pending reaches HTTP 404 and disappears from `render services` within five minutes in a healthy environment; the App finalizer proves the relevant build, registry, credential, TLS, and workload inventory absent before removal. The pre-existing production fixture `srv-d9f9oalju7gs73fvngqg` / `cli-oapi-r-220737-u` is diagnosed and safely removed, with no forced finalizer removal until external cleanup has been verified. Exact POST/PATCH wire regressions, delete-during-build restart/failure tests, verifier self-tests, backend/operator gates, and sanitized production evidence all pass.

## Known live evidence

- On 2026-07-20, official CLI v2.21.0 image create sent the Render-spec-shaped nested image object and received HTTP 400: `bad request: request body contains unknown field "ownerId"`. The OpenAPI request gate accepted the body; the later strict Go decoder rejected `image.ownerId` because `lego/backend/internal/apps/rest.go`'s `imageRef` models `imagePath` and `registryCredentialId` only.
- A short repo-backed fixture successfully completed CLI create, update, readback, and DELETE acknowledgement. After DELETE, deploy and instance reads were empty, but direct GET and eight repeated list reads still returned the App in phase `Deleting` several minutes later. This is not a replica-cache observation.
- Production diagnosis localized the stall to three compounding mechanisms: the first reconcile synchronously polled its build Job for the full 20-minute timeout and could not observe the deletion update; the `bex-build-credentials` Role could delete but not list Secrets/ServiceAccounts and could not delete Pods; and registry cleanup fell back to anonymous because the configured shared push Secret was absent. The old finalizer then revoked the per-App Zot credential despite those failures, removing the least-privilege authority needed to retry. Local m49 changes interrupt the build wait, close only the namespaced RBAC gaps, restore/activate per-App cleanup auth, persist the registry-absence stage, and sequence revocation last. Production recovery remains pending the normal ship/deploy path.
- The deletion result conflicts with completed `w2/m61`'s residue-free finalizer acceptance. This milestone is a focused post-m61 production regression investigation, not a duplicate of the earlier operator audit.

## Source + Goal linkage

- **Source:** user-directed Render CLI production acceptance on 2026-07-20 against `https://api.bex.co/v1/`, using the unmodified official CLI v2.21.0 after the OpenAPI request-validation rollout. Read-only commands and repo-backed create/update/read passed; the two failures above are the bounded handoff scope.
- **Goal linkage:** ADR008 pillar 1 / Render compatibility and the reliable service-lifecycle goals in `docs/ADR004-deployment.md`; this keeps Render's official CLI as bex's fifth surface without building or forking a CLI.
- **Expected outcome:** image-backed CRUD accepts Render's real wire format while preserving workspace authorization and strict unknown-field rejection, and every acknowledged service deletion either converges to absence or exposes a concrete retryable cleanup failure instead of remaining silently stuck.
- **Why now:** the new OpenAPI gate proved the public schema is correct but exposed drift in the handler's second decoder, while the same production run falsified the current CLI checklist claim that deletion always converges through `w2/m61`. Both are live compatibility/reliability regressions on the newly deployed REST path.
- **Gap analysis:** `w6/m14` covers top-level create `ownerId`, not the nested owner emitted inside Render's image schema; `w6/m31`/`m34` cover registry credentials, not this owner field; `w2/m61` covers durable cleanup generally but did not exercise immediate deletion during the first repo build. No open PM item covers either exact failure.
- **Render parity:** included as t006 because the fixes change the public REST/CLI contract and tenant-visible deletion lifecycle; GraphQL, MCP, and dashboard must retain the same core ownership and deletion semantics even though `image.ownerId` itself remains a Render REST wire field.
- **Safety boundary:** never print API keys, kubeconfigs, registry credentials, or raw authenticated response bodies into evidence. Do not forcibly strip the live App finalizer merely to make GET return 404; first prove or complete every external-artifact cleanup step. Do not touch unrelated production resources.
