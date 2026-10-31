# Common generated-image qualification source — 2026-10-31

The common publication now exposes its immutable digest only after the existing runtime, scan, SBOM and signing gates. Two architecture jobs prepare target-runtime wheels, then run the maintained original13-program replay, independent omission checks and exact Python3.14.8 generated-image report binding. Jobs use their own explicit builder and labels and retain evidence before exact owned cleanup. Production pins remain empty.

Review found a real Syft identity defect in the initial source: Docker config ID and SPDX manifest root are different identities. The correction emits full Syft JSON and SPDX from one scan, binds actual config/platform through Syft metadata, and binds SPDX to the same scan's manifest digest. Both complete reports, gzip roundtrips and custody hashes remain available even on a binding failure.

[Root review and custody](ROOT-REVIEW.json) preserve the initial failed source and corrected handoff. Root checked all43 initial and30 corrected frozen payloads and all six live source files/modes. Full build-package tests,41 Python publisher/qualification controls, scoped lint, actionlint and Actions guard pass. Existing full arm64/amd64 reports reproduce the old identity rejection and pass the corrected source; the macOS replay stubs only the Debian version comparator. Logs and patches use lossless gzip to preserve their bytes.

Actual new common generated-image execution and report capture remain unexecuted. This source does not approve an artifact, adopt a production pin, prove all27 runtime selections, close exact-default222 or complete any original28 hosted case.
