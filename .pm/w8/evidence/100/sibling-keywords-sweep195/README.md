# Sibling reference keywords choose unrelated fields — sweep195

The same w8/100 cause applies to ordinary service environment copies, independently of pooling. On 2026-10-13 UTC, each file declares two Free image web services. The caller's second env entry copies a missing key from the declared source. Both published Bex v0.3.2 and unmodified Render2.27/a764810a7682 against Bex correctly reject it, but the missing key text chooses an unrelated caller setting.

| Missing target key | Wrong service field | Field exists in caller? |
| --- | --- | --- |
| IMAGE | image | yes |
| PLAN | plan | yes |
| TYPE | type | yes |
| CONNECTIONPOOL | connectionPool | no |

All four shapes reproduce at root and project-environment placements in both clients: 16 mislocations. Four plain ABSENT_VALUE refusals retain the relevant broad envVars path; four controls with a declared literal IMAGE key validate. All 24 decisions are correct. Merely checking whether a guessed field exists would preserve the twelve wrong existing-field locations.

Four additional native multipart captures contain two bad-field API replies, one plain-missing refusal control and one known-IMAGE positive control. HTTP200 result validity and errors match native output (omitted/null empty positive errors normalize to the same empty list). Eight actual current-source ValidateBlueprint diagnostics, through a virtual test overlay, reproduce the wrong line/field. Those assertions expect the current bug; they do not verify a future fixed source or a full backend suite.

`resolveRef` at deploy.go:2784–2786 returns wording with the missing target env key. `classifyRefs:2104–2107` knows the caller's COPY_VALUE key but wraps only the service name. The shared mapper cannot select that env entry and searches message keywords instead. Type mismatch/addressability errors in the same loop already use envVars["KEY"] context; their existing association must stay intact. Extend w8/100's correction across the untagged reference errors, keeping its pooled-reference branch and direct-admission controls.

All temporary copied configs, capture servers/threads and comparison config are removed. No resource, group, SQL connection or paid feature is applied. Baseline quota is six services, zero terminating and no datastores. Source c5a0eb9dd15f6b417bf586fdf26510706497b032; deployed identity unknown. Non-secret documents/messages remain exact; credential/email values and private paths are redacted. No implementation, commit or push is performed.

Evidence:

- [location-events.jsonl](location-events.jsonl)
- [location-input.json](location-input.json)
- [location-overlay.json](location-overlay.json)
- [location-probe_test.go.txt](location-probe_test.go.txt)
- [location-results.json](location-results.json)
- [metadata-cleanup.json](metadata-cleanup.json)
- [native-capture-driver.py.txt](native-capture-driver.py.txt)
- [native-driver.py.txt](native-driver.py.txt)
- [native-matrix.json](native-matrix.json)
- [native-wire.json](native-wire.json)
- [project-known-image.yaml](project-known-image.yaml)
- [project-missing-image.yaml](project-missing-image.yaml)
- [project-missing-plain.yaml](project-missing-plain.yaml)
- [project-missing-plan.yaml](project-missing-plan.yaml)
- [project-missing-poolword.yaml](project-missing-poolword.yaml)
- [project-missing-type.yaml](project-missing-type.yaml)
- [root-known-image.yaml](root-known-image.yaml)
- [root-missing-image.yaml](root-missing-image.yaml)
- [root-missing-plain.yaml](root-missing-plain.yaml)
- [root-missing-plan.yaml](root-missing-plan.yaml)
- [root-missing-poolword.yaml](root-missing-poolword.yaml)
- [root-missing-type.yaml](root-missing-type.yaml)
- [source.txt](source.txt)
