# w8/m61 — empty and narrow PATH pre-deploy acceptance

Published Bex 0.3.2 and unchanged Render 2.27/a764810a7682 each create two fresh Free Node web fixtures: application PATH empty and PATH=/usr/local/bin. Every actual native create exits 0. All four initial deployment rows become Live and public HTTP 200 proves the exact original PATH plus matching multiline/Unicode environment and file byte hashes at build and runtime. Absolute Node build/start/pre-deploy commands stay unchanged; there is no PATH broadening, recovery deploy or configuration workaround. [Complete native/API/runtime/cleanup receipts](journey-results.json); [guarded fixture driver](fixture-driver.py.txt).

The four exact pairs are:

| Client / PATH   | Service                  | Initial deploy           |
| --------------- | ------------------------ | ------------------------ |
| Bex / empty     | srv-db88hjbh673c73birocg | dep-db88hjbh673c73birod0 |
| Render / empty  | srv-db88hk3h673c73biroeg | dep-db88hk3h673c73birof0 |
| Bex / narrow    | srv-db88hl5eelgc73d2hug0 | dep-db88hl5eelgc73d2hugg |
| Render / narrow | srv-db88hlteelgc73d2hui0 | dep-db88hlteelgc73d2huig |

API-only type=predeploy reads independently recover exactly one JSON program output for Bex/empty and both Render cases. Each verifies the same exact PATH and variable/file hashes in the actual pre-deploy process. The pinned CLI exposes no predeploy log type; these are explicitly API diagnostics, not native predeploy-log coverage. [Driver](api-log-driver.py.txt).

**Direct output limit:** Bex/narrow reaches Live/HTTP 200 with exact build/runtime bytes, and its exact row has durable pre_deploy_started and pre_deploy_ended/succeeded events. Its pre-deploy command is present unchanged in service metadata. Two marker queries and an unfiltered predeploy query return 200 with no rows. No actual pod/store error or cause is observed; ephemeral Job logs are not proof that the command was skipped. Direct pre-deploy byte output for this one case remains unobserved. The other three records cover both PATH classes and both clients, while the four native lifecycle/runtime checks and the source's unchanged Env/EnvFrom establish the startup gate. No second deploy is issued just to obtain a log. This is recorded as a coverage limit, not a new bug.

An earlier native Render query used type=app and correctly returned empty output because application and predeploy streams are separate. The QA parser tried to decode that empty output as JSON. Its unchanged native receipt is recovered from the private runner journal and retained in the resource record; the [original probe](excluded-app-log-probe.py.txt) is excluded as predeploy coverage. No unsupported CLI type is invoked or filed as a product failure.

Source `667943d859ff71ab3f6a085985dcd5b4e272e0c5` includes327c8ab9b. The [pin inspection](pin-scope.json) confirms the new5c7ee1e5aae6 image-pin commit updates the operator Argo image as well as the API deployment; an earlier summary describing only the API was incomplete. This is checked-in deployment intent, not observation of the deployed artifact. `predeploy.Job` now emits /bin/sh -c with the application Env/EnvFrom unchanged, and the controller passes the in-flight release's file mounts. Prior worker checks cover the normal/omitted neighbors and lifecycle roles; earlier milestone evidence covers the other native/runtime/literal boundaries and retains its stated limits.

All four exact-owned services and four enumerated deployment children are cleaned: native deletion success, detail404, list absence, former runtime404, six baseline services present, quota used6/terminating0. [Cleanup](cleanup-driver.py.txt) and [final reconciliation](reconciliation-driver.py.txt) preserve first pending quota observations, never repeat deletion, and remove exact owned local inputs/config copies. No direct production Kubernetes descendant read is claimed. Hosted literal Blueprint writes, non-REST write parity, independent Render production and exact deployed revision remain limited as previously documented; this last gate is the four native pre-deploy startup cases. No paid resource, product edit, deployment, release, commit or push by QA.

This completes the explicitly parked live gate for [m61](../../README.md). All eight task statuses, the closeout move and workstream status are synchronized. Historical failed/recovery evidence remains intact.
