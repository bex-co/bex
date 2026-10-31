# Private build Python for legacy Node14/16

Legacy Node addon tooling needs a Python API older than the selected modern app Python: node-gyp5 uses the removed rU file mode, and node-gyp9 imports distutils. Common and historical Bookworm Node14/16 primary/companion paths now reuse the reviewed Python3.9.25 Bookworm donor under the private prefix`/opt/bex/native-node-python`. The finder inserts that default after upstream explicit choices and before automatic Python candidates. Application Node/Python versions, environment, prefixes and PATH remain unchanged; raw Bullseye paths retain their existing compatible distribution.

The shared helper verifies relocated stdlib/extension closure including distutils. Historical recipes embed the same helper bytes. The production Docker context explicitly includes the embedded asset, and operator tests compare both copies, inspect context inclusion/exclusion, and exercise canonical companion npm-before-helper-before-authored ordering. Invalid forced Python still stops; ordinary failed config/PYTHON candidates keep upstream fallback, and accepted explicit modern Python is respected.

Five host helper controls pass, exercising the genuine retained node-gyp5/9 finder methods and precedence/default cases. Independent final source review is CLEAN and binds all ten files, including the Docker context and follow-on drift guard. The first full operator attempt passed the build and controller packages but failed a disk mode test because this worker's macOS temporary directory inherited the wheel group. A bounded host probe established the group-dependent setgid behavior; assigning only the new owned temp root to the effective group makes the original disk test pass. Repository disk code is unchanged. The corrected full operator suite and scoped lint are running at this dated checkpoint; their actual results will be appended.

The source/host capsule preserves initial, formatting and follow-on generations, the actual failed local attempt, its diagnosis and focused correction evidence as gzip with exact original identities in[CUSTODY.json](CUSTODY.json). Actual generated images, private-Python/addon/ABI checks on both architectures, common artifact adoption and hosted acceptance remain open under[m91/t007](../../t007.md). No production pin changes are included.


## Final local verification — 2026-10-31

The corrected full`make test` passed in264.826s, including code generation, vet, all operator packages and the real local envtest controller suite. The original disk-mode regression also passed without repository changes. All observed private envtest processes ended and owned temporary trees were removed.

Scoped lint then reported the resolver's actual complexity31. Centralizing the common/historical mode guard inside the existing helper removes duplicate caller branches and adds a common-mode exclusion assertion. The complete build-package tests passed again (6.327s), followed by scoped lint with zero issues; all final source hashes stayed unchanged during verification. The broader full suite was not repeated after this small behavior-preserving helper/guard refactor.

A first targeted compile preflight retained the full-suite allocation allowance and refused before starting any child. The final targeted run uses a smaller512MiB own allocation limit, based on the measured full-suite peak352,652,309bytes, preserving the1GiB physical reserve and128MiB margin; its actual peak was43,143,853bytes with no resource stop. These preflight and lint failures remain distinct from the actual passes. Final raw diffs, source bindings and execution receipts are included in custody.


The final three-file mode-guard refactor also received independent source review:CLEAN. It binds all final source hashes, verifies the other seven owned files and helper-copy equality, and confirms that both callers preserve order, arguments and errors while common mode returns no historical lines.


## Rebased verification — 2026-10-31

A concurrent worker shipped Docker Hub metadata retries in the same build package. After integrating those two files and the separately shipped GHCR qualification fix`c13c25236`, the complete build-package tests passed again (6.280s), followed by zero-issue scoped lint. Final helper/fixture and incoming retry-source hashes were unchanged; owned temporary files were removed. This recheck covers the new package context without repeating unrelated controller tests.
