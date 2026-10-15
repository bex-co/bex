# Native Node empty-PATH runtime failure — sweep216

Four fresh Free Node web fixtures separate build preparation from image startup. A fresh Bex checkout with PATH omitted and published Bex with normal PATH reach Live/HTTP200 with exact build/runtime PATH, 30-byte marker and 24-byte file hashes. Pinned Render and published Bex both accept an explicitly empty PATH and finish the build, but runtime startup fails for docker-entrypoint.sh before the absolute Node command runs. Render reaches update_failed; Bex is captured stalled, then superseded by the recovery control. Bex's original terminal failure is not claimed.

Both exact-owned PATH-only save-only controls, followed by native deployment of each original commit, reach Live/HTTP200 with the original commands and matching marker/file bytes. A temporary Bex Live/public503 discrepancy converges, and one status read has a connection reset; neither is filed as a separate bug. Four services and six enumerated child deploys are deleted, with detail/list/former-public absence, all six baseline IDs present and final quota 6/PG0/KV0/termination0. Own input files and Render comparison configuration are removed.

Primary OCI metadata for the exact source-pinned Node image index proves both Linux architectures declare relative ENTRYPOINT docker-entrypoint.sh and copy it into /usr/local/bin. The native generator inherits ENTRYPOINT while replacing CMD with absolute /bin/bash and the authored command. The native serving container keeps that image entrypoint. Empty runtime PATH prevents the inherited program's lookup; RUN shell tests never exercise it. The deployed artifact configuration and deployed SHA are not directly inspected, so attribution combines actual startup events, both causal controls, exact pinned base metadata and current producer/consumer source.

Five diagnostic executions of the actual generated preparer/loader pass literal-over-stored PATH precedence, including empty values. Four named build tests and five named controller projection/cache/file tests pass. These local results cover the build boundary; they do not prove image startup or a full suite. No product source is edited.

This is the unfulfilled empty-PATH runtime gate already in m61, not a duplicate milestone. Add t007 for the inherited native entrypoint, reopen final quality/regression work for the expanded implementation, and retain historical completed build-loader work. No QA implementation or ship.

Native create/deploy wire was not captured. Full native commands/stdout/stderr/exit/time and actual same-authority event GETs and PATH-only setup replies are retained in the fixture ledger. Private paths and the QA actor ID are redacted. The oversized fixture name and OCI reader setup errors are excluded correct refusals.

- [Complete owned native/runtime/cleanup ledger](owned-fixtures-native-runtime-cleanup.json)
- [Metadata and explicit limits](metadata-cleanup.json)
- [Primary entrypoint metadata](entrypoint-metadata.json)
- [Linux amd64 configuration](node-linux-amd64-config.json)
- [Linux arm64 configuration](node-linux-arm64-config.json)
- [Initial startup event reply](initial-Render-startup-events.json)
- [Current source census](source-census.txt)
- [Build test summary](build-test-summary.json)
- [Controller test summary](controller-test-summary.json)
- [Five generated-shell precedence cases](literal-precedence-cases.json)
- [Native fixture/observer driver](fixture-observer-driver.py.txt)
- [PATH-only recovery driver](PATH-only-recovery-driver.py.txt)
- [Exact-owned cleanup driver](cleanup-driver.py.txt)
