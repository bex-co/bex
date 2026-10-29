# Seed-once sibling copies after edit/reapply — sweep214

Five actual public apps/secrets feature cases over local fake Kubernetes/versioned storage produce three configured source/copy mismatches. A sync:false seed is initially applied, edited through normal SetEnvVar to edited B, then the Blueprint is revalidated and reapplied. SeedEnvVars correctly preserves edited B; GetEnvVar confirms it. The consumer's persisted COPY instead receives seed A, empty, or changed declaration declared C. Unedited seed and immutable manifest-literal controls agree. No hosted edit/apply/pod or live authorization is claimed; the harness is CR-only legacy mode and uses its object-name storage selector.

The actual shared validator admits the reapplied document despite the local existing state. Both published Bex 0.3.2 and unchanged Render 2.27/a764810a7682 admit all 10 corresponding fresh declarations in production; two exact unchanged native multipart captures return HTTP 200/valid:true with decoded agreement. These production requests prove declaration admission only, not existing-state behavior. Authorization is omitted. The local source HEAD before/after is e2cc474297bac5212e5d64e2e251377e738b2006. Current linked-group and shadowed-reference neighbor tests also pass; no full suite is claimed.

apps/deploy.go:2519–2525 adds provided sync:false seed hints to se.known, which knownServiceEnv overlays and parseCompiledStack publishes before copies resolve. secrets/service.go:664–665 intentionally retains any already-present key during seeding; its preservation is correct. On reapply the parser's hint is no longer proof of the source's configured value. Existing m64 already corrects the same known-index assumption for group state, so this extends that milestone rather than filing a duplicate or undoing seed-once retention. Existing w8/092's empty presence correction stays valid.

Fake App IDs are not production ownership/deletion targets. No hosted resource, paid feature or runtime value is created. All copied native configs/capture servers/threads/children are removed, quota is baseline 6 with PG/KV0 and terminating0. The initial public-ID-versus-legacy-name selector mismatch is retained privately and excluded; it is a QA setup failure, not a reported product defect. Real-DB/public-ID, hosted apply/reapply/runtime, snapshots, prompt overrides and non-REST behavior remain untested. No QA implementation/commit/push/release.

Evidence:

- [actual-backend-results.json](actual-backend-results.json)
- [backend-probe.go.txt](backend-probe.go.txt)
- [case-0.yaml](case-0.yaml)
- [case-1.yaml](case-1.yaml)
- [case-2.yaml](case-2.yaml)
- [case-3.yaml](case-3.yaml)
- [case-4.yaml](case-4.yaml)
- [metadata-cleanup.json](metadata-cleanup.json)
- [native-capture-driver.py.txt](native-capture-driver.py.txt)
- [native-matrix.json](native-matrix.json)
- [native-wire.json](native-wire.json)
- [shared-fake-KV-and-pointer-helper.go.txt](shared-fake-KV-and-pointer-helper.go.txt)
