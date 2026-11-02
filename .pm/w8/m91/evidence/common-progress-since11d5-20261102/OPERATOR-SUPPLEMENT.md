# Completed local operator verification

Root admitted the sealed local check on source `3a15c2a`: unchanged `make test` passed in 341.808 seconds, and configured `./internal/build` lint passed in 6.030 seconds with zero issues. Total controller time was 348.258 seconds. This verifies the unapplied future data and test fixture.

All 13 preparation payloads and 26 output identities are retained. The archive contains 25 output bodies, including complete make and lint output, process records, source and view comparisons, and temporary-directory metadata. Generated `cover.out` remains private with its exact identity. Git preimages and manifests reconstruct the 1,147 input files and the 1,148-file final view. All 17 overlays and 301 external source inputs stayed unchanged.

The harness changed only its new temporary directories: it set their group from 0 to the process’s primary group, 20, after checking owner and inode identity. Owner 501 and mode 0700 were preserved. The process profile allowed setting mode bits only beneath the new TMP directory. Product restore code, ownership and mode assertions, and the other process restrictions stayed unchanged.

Both direct process groups and 12 additional observed groups were absent after the run; owned TMP and HOME were removed. These observations cover selected process ancestry. Complete raw stderr retains the nonfatal Xcode warnings.

The earlier failed operator run and all three probes remain unchanged. This supplement adds the later local result to the first packet’s historical snapshot. Its SHA and mode map reuses exact existing objects and retains all new bytes. Binaries, shared caches, temporary journals, physical source trees and the coverage body are omitted explicitly.

The result does not establish run 97 utility admission, remaining family results, production adoption, deployed readiness, Free or authentication checks, or the original hosted acceptance. See `OPERATOR-SUPPLEMENT-RESULT.json` and `OPERATOR-SUPPLEMENT-MEMBER-MAP.json` for the exact result and custody.
