# Own mutable overrides remain outside the Blueprint copy index — sweep219

Current apps/envgroups/secrets methods create a source linked to a literal group and a consumer copying VALUE. A normal own-env edit adds VALUE after the initial apply. Both validation and reapply still succeed, but the source's persisted final own Secret contains edited own or empty, while the consumer COPY remains group A. The key names seam actually reports VALUE in both cases. Unrelated own key, explicit manifest-literal precedence and no-own-store controls all agree.

Five diagnostics use real feature methods over fake versioned storage/Kubernetes. Effective process values are inferred from actual persisted group-first/own-last/literal-highest precedence; no hosted pod, auth, apply, release-snapshot or current-state production claim is made. No fake IDs are entered in production ownership ledgers or used as deletion targets. The diagnostic passing exit means it executed, not that the product is fixed.

Eight fresh-declaration validations through published Bex and unmodified Render admit group/literal/empty/unrelated spellings. These requests have no existing source carrying the edited own value. They prove ordinary CLI admission only; no native HTTP capture is claimed and they do not certify existing-state eligibility. No hosted resource, comparison config or product source edit remains.

The new source guard handles seeds and effective group order but leaves the own-store override unguarded. Inventory the specific target key through the already-wired names-only seam before deriving a group copy; refuse at the consumer's authored entry when that own value is unknown, without reading values. Preserve manifest literals, unrelated own keys and first-create/group/seed repairs. Existing m64 is reopened with t007; prior shipped receipts and the complete original099/group/seed DoD are preserved.

- [Actual feature method states](actual-feature-method-results.json)
- [Current probe](current-feature-probe_test.go.txt)
- [Fake storage helper](fake-storage-helper-and-prior-probe.go.txt)
- [Fresh native admission](native-fresh-admission.json)
- [Native driver](native-admission-driver.py.txt)
- [Metadata and explicit limits](metadata.json)
