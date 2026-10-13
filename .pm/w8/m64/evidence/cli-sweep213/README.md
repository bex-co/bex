# Group-derived copies versus persisted link order — sweep213

Actual apps and envgroups services create/reapply seven cases over local fake Kubernetes and versioned storage. Four persist a consumer copy different from the source value implied by its projected group order; unchanged order, own literal and own empty controls agree. The initial persisted objects are retained, so the A,B,A duplicate case is also proven on first apply. Seven actual operator envFromSources/appEnv projections preserve those orders and literal overrides. Kubernetes API v0.35.0 core/v1/types.go:2953–2962 specifies last envFrom source precedence and explicit Env override. Pod consequences are inferred from that primary contract; no controller reconciliation, container, active release snapshot or hosted apply is executed.

Published Bex 0.3.2 and unmodified Render 2.27/a764810a7682 admit all 14 fresh declarations corresponding to those desired manifests. Four unchanged native multipart POST /v1/blueprints/validate captures cover duplicate links in both clients, a reordered declaration and the own-empty control: HTTP 200/valid:true, decoded native/API agreement, Authorization omitted. These prove eligibility of the shapes, not existing-state behavior in production. Existing-state mismatches are the actual local public DeployStack/ValidateBlueprint evidence, clearly separated here.

Source HEAD before/after both local runs is d15c1e7fcfe0bc4806f398c9b94535560380e837. The probes expect present bugs, not corrected regression behavior. TestAppEnv's four subcases and TestEnvFromSources pass; no full backend/operator suite is claimed. Earlier six-case proof was expanded with duplicate links and first-apply objects. An initial wrong legacy selector and an operator port-type compile failure are retained privately and excluded.

Fake App/group IDs are diagnostic objects and never production ownership or deletion targets. The native runs apply nothing; no hosted resource or paid feature is created. All config copies/servers/threads/native children are removed, quota remains baseline 6 and PG/KV0/terminating0. Real-DB/public-ID mode, runtime/current-state native/API/non-REST acceptance and actual deployed revision remain unverified. This researched continuation promotes existing 099 into m64; no duplicate implementation issue, QA source fix or ship occurs.

Evidence:

- [actual-backend-results.json](actual-backend-results.json)
- [actual-operator-results.json](actual-operator-results.json)
- [backend-probe.go.txt](backend-probe.go.txt)
- [case-0.yaml](case-0.yaml)
- [case-1.yaml](case-1.yaml)
- [case-2.yaml](case-2.yaml)
- [case-3.yaml](case-3.yaml)
- [case-4.yaml](case-4.yaml)
- [case-5.yaml](case-5.yaml)
- [case-6.yaml](case-6.yaml)
- [metadata-cleanup.json](metadata-cleanup.json)
- [native-capture-driver.py.txt](native-capture-driver.py.txt)
- [native-matrix.json](native-matrix.json)
- [native-wire.json](native-wire.json)
- [operator-input.json](operator-input.json)
- [operator-probe.go.txt](operator-probe.go.txt)
- [source.txt](source.txt)
