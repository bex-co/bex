# Complete synthetic catalog: full operator suite

On 2026-10-30, the normal operator `make test` passed with exit 0 in 234.906 seconds on source `a984ac53ffabfa55c5596e30fecbd8b7500804ed`. A Go overlay replaced only `nativebases.go` for the test process with a complete twenty-seven-entry catalog keyed by the current reviewed official/upstream bases. Every replacement reference is explicitly synthetic under `example.invalid/bex-synthetic/`. The real production source and empty catalog were never changed.

The full suite includes CRD/RBAC and deepcopy generation, formatting, vet, all normal operator packages and controller/envtest. `internal/build` passed in 8.316 seconds, `cmd/native-resolve` in 1.109 seconds and controller/envtest in 217.980 seconds. Image integration gates were unset, so these fake references were not pulled or executed. This result checks populated-catalog assumptions in the broader suite; it is not supplier or runtime-image verification.

The before/after hash maps contain the same 550 tracked source, CRD and RBAC files, with zero differences. Those hashes were independently rechecked against the current files during archival. The original receipt, overlay, replacement source, reviewed source inputs and catalog rows are retained byte-for-byte. The full raw log is losslessly gzip-compressed with both compressed and uncompressed hashes in MANIFEST.json. The raw log hash matches the original receipt.

Go sources use `.go.txt` archive names to prevent package discovery. `overlay.json` retains the original absolute audit paths, including the former scratch replacement path; it must be remapped to an appropriate scratch copy for replay, rather than used as a production configuration. No source or generated output was restored, and unrelated workspace checksum changes were left untouched.

The production supplier catalog remains empty. Published multiarch artifacts, signatures/SBOM review, actual adoption and the original fresh hosted acceptance remain separate open work. No m91 task or milestone is closed by this synthetic suite.
