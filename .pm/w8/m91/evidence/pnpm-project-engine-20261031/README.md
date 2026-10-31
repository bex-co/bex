# pnpm project-engine fixture correction

The qualification fixture now tests pnpm's project engineStrict path with an explicit workspace root. pnpm10.34.7 accepts a cold local tarball dependency without checking its engines; using that dependency as the negative test incorrectly rejected the genuine runtime. npm6 and Yarn keep their local-dependency controls. All nine positive manifests remain byte-identical to the previously shipped fixture. The new oracle requires exit1, exact project CWD, wanted range and full selected Node patch; it refuses unrelated failures, changed identities and signal exits.

Nine host fixture controls pass, independently reviewed. The bounded V4 runtime calibration then used genuine official Node14.21.3/16.20.2 and unmodified official pnpm10.34.7 on arm64. Both actual project-engine controls returned exit1 with the precise CWD, selected full version and expected range. Both cold tarball dependencies were accepted exit0. The fixture correction uses the same project-engine path with its selected next-major range; the opt-in full common-image fixture has not yet run.

The calibration also observed actual lifecycle scripts, missing default node-gyp and NODE/npm_node_execpath pointing to the standalone executable; explicit NODE comparisons ran. These are unwrapped-binary observations, separate from the already-shipped production launcher. The strict effective-config oracle saw omitted default keys as null and remains unqualified; overall standalone viability is FAIL. Offline package-manager-selection timeout is incomplete. No C-extension compile, common image, amd64, production adoption or hosted acceptance is claimed.

Both exact owned containers were deleted; all baseline container IDs remained, with no concurrent additions/removals, and the immutable base image was unchanged. Full input archives remain private and unchanged; their second-run copies were freshly hash/SRI/size checked. All original raw source, independent reviews, incomplete controls, runtime outputs and cleanup identities are gzip-preserved in[CUSTODY.json](CUSTODY.json). No follow-up run or gate relaxation is implied.


After execution and exact container cleanup, six duplicate archive/extracted pnpm files were replaced with exact-hash symlinks to the untouched first-run retained originals, reclaiming132,341,760bytes. The second run used physical copies during execution; all original bytes remain readable. The separate post-completion receipt records this storage change without rewriting execution history.


## Operator verification — 2026-10-31

The corrected fixture's host controls also ran through the passing full operator suite and final build-package rerun. [Execution records](../legacy-node-build-python-20261031/README.md#final-local-verification--2026-10-31) retain the actual full-suite pass and zero-issue scoped lint. Opt-in common-image and hosted qualification remain open.
