# Empty PATH reaches Node startup but still stops pre-deploy — sweep228

Published Bex and the unmodified Render pin each create a fresh Free Node service with PATH explicitly empty. Both finish the original build and reach Live/HTTP200 with empty PATH still present at build and runtime and exact harmless environment/file hashes. A labeled checkout-build omitted-PATH control also passes. This completes the original two-client empty-PATH image-startup gate; actual deployed revision remains unknown.

Two new fresh native creates add only a harmless absolute Node pre-deploy command. Both build, then end pre_deploy_failed before any workload can serve (HTTP404); the build progress names exit128 and the original pre-deploy query has no program output. Changing only PATH through exact-owned save-only API setup and natively deploying each original commit produces Live/HTTP200, exact variable/file hashes and one expected pre-deploy marker for each fixture. Commands, source commit and inputs remain unchanged. This exposes a separate pre-deploy shell lookup inside the same PATH milestone.

The current Job constructor emits [sh,-c,command] and passes application env/envFrom unchanged. Its single controller caller supplies the same env as the service. The controller's documented platform shell policy already uses absolute /bin/sh; the native generated CMD also uses an absolute shell. Source plus PATH-only same-commit recovery strongly identify the remaining relative lookup. The exact deployed image config and OCI startup error were not inspected, so the precise exit128 origin is qualified rather than presented as direct container inspection.

Native command argv, stdout, stderr, exit, histories and HTTP readbacks are preserved. Native production POST bodies were not intercepted. Save-only setup and pre-deploy log readbacks are REST diagnostics, not native CLI coverage. The pinned native client rejects type predeploy (accepts app/request/build); that excluded setup error is retained separately. Three focused existing operator suites pass; neither Docker image execution nor a full operator suite is claimed.

All five exact-owned fixtures and seven recorded deploy children are reconciled, detail/list/former runtime absence is verified, the six baseline IDs remain and service quota reaches6/terminating0. Local note inputs and copied Render configs are removed. Kubernetes descendant teardown remains unobserved. No product edit, QA commit/push, release, paid purchase or external issue submission.

- [Native journeys, PATH-only recovery and cleanup](owned-native-runtime-cleanup.json)
- [Fixture driver](fixture-driver.py.txt)
- [Recovery driver](recovery-driver.py.txt)
- [Cleanup driver](cleanup-driver.py.txt)
- [Passive reconciliation driver](reconciliation-driver.py.txt)
- [Successful pre-deploy marker readbacks](recovery-predeploy-marker-logs.json)
- [Original build failure and empty pre-deploy reply](initial-build-and-empty-predeploy-logs.json)
- [Excluded native log-type setup](excluded-cli-log-type-setup.json)
- [Current source producer/consumer census](source-census.txt)
- [Focused operator suite events](focused-operator-events.jsonl)
- [Versions and limits](metadata.json)
