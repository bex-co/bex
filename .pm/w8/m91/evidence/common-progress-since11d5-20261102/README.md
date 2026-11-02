# CI and local verification evidence — 2026-11-02

All three attempts of utility run 95 failed. The first stopped during the Syft download, the second timed out reading Python 3.14.8 image metadata before its generated build, and the third exhausted three 180-second BuildKit pulls. Complete logs, metadata and both retry requests are retained. No run 95 artifact ZIP body was downloaded. The logs do not establish an underlying CDN, TLS, authentication or rate-limit cause.

Commit `3a15c2a` shares the existing 570-second bootstrap budget across pull attempts while preserving the pinned image, output limits, cancellation and cleanup. The precise old-helper regression fails as expected; all 165 controls pass with the fix. The exact source, checks, ship and run 97 dispatch records are retained. The separate static source review verifies unchanged image and native-init inputs. Run 97 was still active when this packet was prepared.

[The later operator verification](OPERATOR-SUPPLEMENT.md) passes the unchanged full make target and configured build-package lint on the unapplied two-data-file and 15-test-file proposal. It preserves the earlier full-suite failure and all three directory-mode probes. The first probe completed but did not support the group hypothesis; the later ordinary and scoped-profile comparisons did. Compiled binaries, caches, temporary journals, physical source views and generated coverage bodies remain private, with exact identities recorded.

`SELECTION-MAP.json` maps 543 original records to exact public Git bodies or 349 objects in `unpublished-progress.tar.gz`. Original bytes and permission modes are retained. Public body reuse provides storage without changing any recorded execution source or outcome. The operator supplement has its own archive and member map. Private paths in these maps record provenance; the archives are available in this directory.

The frozen selection predates the later operator result, so its null outcome fields remain unchanged. The supplement records that subsequent admitted result separately. `STALE-CLAIMS.json` explains earlier preparation wording. In companion-case shorthand, 14.21.2 is the Node version used with Ruby.

Current utility admission, remaining family checks, production adoption, deployed readiness and the original 28 hosted cases remain open. This evidence does not fill candidate, contract, Free or authentication authorities.
