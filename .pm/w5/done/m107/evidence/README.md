# w5/m107 local runtime verification

Executed 2026-10-02 04:49–04:56 UTC against isolated dev-5 (API :54050, dashboard :50050). Disposable service `srv-davjeh1jg4r1kicjkeug`, App `tea-davjdk1jg4r2nk99cil0-w5-m107-rollback`, workspace `tea-davjdk1jg4r2nk99cil0`. All stages used the same `docker.io/library/busybox:1.36` image, exercising a configuration-only rollback.

| Stage | Generation | Actual ready Pod output | Saved GraphQL command / env / file | Undeployed changes |
| --- | --- | --- | --- | --- |
| A | 3 | `command=A env=env-A file=file-A` | A / A / A | false |
| B | 7 | `command=B env=env-B file=file-B` | B / B / B | false |
| REST rollback A | 8 | `command=A env=env-A file=file-A` | B / B / B | true |
| REST Restart | 9 | `command=A env=env-A file=file-A` | B / B / B | true |
| Delete and replace the ready Pod | 9 | `command=A env=env-A file=file-A` | B / B / B | true |
| Restart scoped reconciler process | 9 | `command=A env=env-A file=file-A` | B / B / B | true |
| Standard deploy | 10 | `command=B env=env-B file=file-B` | B / B / B | false |

A = `dep-davjeh1jg4r1kicjkf10`; B = `dep-davjft1jg4r3s5qqcf0g`; rollback = `dep-davjg4hjg4r3s5qqcf20`; Restart = `dep-davjgl1jg4r3s5qqcf40`; next standard deploy = `dep-davji11jg4r3s5qqcf8g`. Every captured deploy reached live and every captured marker came from the actual ready Pod's generated document. Both scoped reconciler process runs exited PASS after their explicit stop files.

REST env-var/file content, GraphQL env/file reads, MCP `list_env_vars` / `get_secret_file`, and dashboard reveal controls all retained B during rollback. GraphQL `startCommand` retained command B. The existing image-backed REST/MCP service projection and dashboard Settings omit the command field/editor; those surfaces were not claimed to display it. The dashboard showed “Saved changes aren't live yet. Use a standard deploy to apply them.” during rollback and Restart, then removed that notice after the standard deploy. A rollback request targeting the currently live standard deploy returned 409.

## Harness limits and accommodations

- Used the real dev-5 API/store/secret paths and Kubernetes Deployments/Pods. Reconciliation ran a temporary Go overlay loop restricted to the exact fixture namespace (plus dev-5); it did not replace or restart the shared manager. The fresh process reconciled generation 9 at 04:54:06 UTC before persistence evidence was captured. Manager-watch behavior is covered by repository tests, not this polling harness.
- Updated the local additive App CRD schema. No production or other workstream workload was modified.
- There is no public ingress for this fixture; proof uses Kubernetes readiness and actual Pod document content, not an onbex.co request.
- A disposable OpenBao and two dedicated service accounts lived only in dev-5-auth. Its policy covered the fixture workspace, plus read/list of the empty legacy `default/env-groups` index needed by the existing read fallback. Cleanup briefly added the identity's auto-created empty workspace's exact paths. No shared secret store or agent stack was used.
- Prebuilt images reject an auto-deploy declaration, so this fixture did not test the dashboard/API auto-deploy difference. That behavior requires adapter/regression coverage, not a claim from this live fixture.
- Browser signed in as a disposable, verified local identity. Authentication values and token files were removed on cleanup. Evidence includes only non-secret marker contents.

## Cleanup

REST service deletion succeeded; both the explicit fixture workspace and its auto-created default workspace were deleted through GraphQL; the disposable Kratos identity was deleted. The scoped reconciler stopped, and OpenBao Deployment/Service, both service accounts, TokenReview binding, forward, token file and private auth-state file were removed. Standard dev-5 API configuration was restored and `/healthz` returned `ok`.

Final cleanup verification: Kubernetes namespace deletion completed; no fixture App/Deployment/ReplicaSet/Pod/Secret/ConfigMap/Service remained. The scoped reconciler process was absent and standard dev-5 API still returned `ok`.
