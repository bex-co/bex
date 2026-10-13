# Shadowed literal environment copies — sweep204

Published Bex0.3.2 and unmodified Render2.27/a764810a7682 against Bex accept four mixed literal/reference shapes in both clients. Four positive controls (unique keys and literal-only duplicates) validate; four pure-reference-chain controls are correctly refused by Bex's existing plainly-defined-value restriction. Native validation only: no hosted apply or runtime was attempted.

Six exact native multipart captures establish three accepted problematic shapes, two valid controls and one pure-chain refusal; all HTTP200 and native validity/errors equal the API data. Harmless file bytes and own ownerId are forwarded unchanged; Authorization is omitted.

Six actual current-source public ValidateBlueprint -> DeployStack diagnostics write App specs into a fake Kubernetes client. Four persist inconsistent source/copy values (both entry orders, consumer-first declaration, and an empty losing literal); two controls persist matching values. The recorder's creation seed maps are correctly empty: Blueprint literals remain manifest-owned on App.Spec.Env. The final diagnostic reads those actual persisted arrays. Its map interpretation follows the reviewed kubelet behavior; it is source-backed inference, not observed hosted pod data.

The earlier diagnostic incorrectly looked for literals in create seed maps. That QA-only failed attempt is excluded and retained separately. The initial native driver snapshot contains that original probe generator; use the final apply-probe/input/overlay and the exact command in apply-results for the corrected diagnostic. Assertions expect the current mismatch, not a future fixed regression. Four existing neighboring tests pass; no full backend suite is claimed.

No hosted service, grouping, datastore or paid feature was created. All copied configs and forwarding servers/threads/child processes are removed; baseline quota remains six services, zero terminating and no datastores. Current source c5a0eb9dd15f6b417bf586fdf26510706497b032; live API/operator/Kubernetes revisions unknown. Credentials/email and private paths are redacted. No implementation, commit, push or external issue filing.

Evidence:

- [apply-events.jsonl](apply-events.jsonl)
- [apply-input.json](apply-input.json)
- [apply-overlay.json](apply-overlay.json)
- [apply-probe_test.go.txt](apply-probe_test.go.txt)
- [apply-results.json](apply-results.json)
- [consumer-first-pure-chain.yaml](consumer-first-pure-chain.yaml)
- [consumer-first.yaml](consumer-first.yaml)
- [empty-shadowed-literal.yaml](empty-shadowed-literal.yaml)
- [excluded-diagnostic-seeder-assumption.json](excluded-diagnostic-seeder-assumption.json)
- [literal-then-reference.yaml](literal-then-reference.yaml)
- [literals-only.yaml](literals-only.yaml)
- [metadata-cleanup.json](metadata-cleanup.json)
- [native-capture-driver.py.txt](native-capture-driver.py.txt)
- [native-driver-initial.py.txt](native-driver-initial.py.txt)
- [native-matrix.json](native-matrix.json)
- [native-wire.json](native-wire.json)
- [primary-contract-review.json](primary-contract-review.json)
- [pure-reference-chain.yaml](pure-reference-chain.yaml)
- [reference-then-literal.yaml](reference-then-literal.yaml)
- [source.txt](source.txt)
- [unique-control.yaml](unique-control.yaml)
