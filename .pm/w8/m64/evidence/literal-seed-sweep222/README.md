# Immutable literals are conflated with same-key seeds — sweep222

Eight diagnostics call actual apps/secrets feature methods over fake Kubernetes and versioned storage, with the consumer declared first. A source declares VALUE as an ordinary immutable literal and also as a sync:false seed. Both entry orders are accepted. The persisted source's explicit Env wins over its own Secret, as the current operator and ADR013 specify. If the seed comes last in the authored list, the accepted first apply instead copies seed into COPY, while the source uses fixed or empty. Both entry orders are then falsely refused on reapply as an unknowable seed, despite the known immutable literal. This is two first-apply mismatches and four false refusals.

Generated/literal pairs in both orders and a plain literal agree and reapply successfully. The seed-only first apply agrees, and its existing-source refusal is intentionally correct. Process values are inferred from actual persisted projection; no hosted pod, apply, release-snapshot or authentication claim is made. The diagnostic passing exit records execution, not fixed product behavior or a full suite.

Sixteen fresh native validations through published Bex and the unmodified Render pin all admit the declarations with exit0/valid:true. Every command has its exact unchanged multipart request and response, with auth omitted. Fresh plans do not demonstrate source/copy values or existing-state eligibility in production. No hosted resource, forwarding server or copied config remains.

The existing m64 known-value contract owns this gap. New t008 separates immutable literal priority from seed metadata; t007's own-store guard and the complete earlier group/seed/original099 scope remain required. No duplicate issue, product source edit, commit or push by QA.

- [Eight actual feature states](actual-feature-method-results.json)
- [Current diagnostic](current-feature-probe_test.go.txt)
- [Fake storage helper](fake-storage-helper-and-prior-probe.go.txt)
- [Diagnostic events](diagnostic-events.jsonl)
- [Sixteen native commands and original wire](native-fresh-wire.json)
- [Native capture driver](native-capture-driver.py.txt)
- [Versions and limits](metadata.json)
