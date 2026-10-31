# PostgreSQL fixture timestamps

The full real-dependency backend gate reported three failing store tests: webhook display names, a concurrent workspace-deletion audit and account-cleanup eligibility. Their fixtures compared or ordered timestamps from the host clock against PostgreSQL-created timestamps. The first two legacy-name paths deliberately reject events preceding the current app's creation; a test intending to exercise the current app must establish that condition explicitly. The account cleanup worker likewise uses the database clock for expiry and retry leases.

These three test-only corrections use database-returned app/tenant creation timestamps or precise PostgreSQL clock snapshots. They preserve exact display-name expectations, observed blocking-PID synchronization, retained audit/projection counts, immediate cleanup eligibility, retry leases, provider correlations and actual claim/retry operations. Production code, migration bodies and stale-event guards are unchanged. The existing migration round trip already rolls back its transaction; it was not rewritten. No persistent clock offset or schema contamination is claimed.

All seven focused real-DB tests and eight subtests passed with zero skips. These include the three affected tests, migration round trip, stale reused-name generations, deleted-ID-as-name exclusion and migration ownership boundary controls. Scoped store lint and formatting pass. The complete backend gate remains required before pushing; its actual final outcome will be appended after execution.

Original failing-suite, source, clock-read and focused-validation records are preserved losslessly as gzip, with exact byte identities in `CUSTODY.json`. Resource precheck/amendment records distinguish incomplete attempts from actual test results.


## Full backend gate and shipment — 2026-10-31

The complete CI-pinned real Postgres/OpenFGA/OpenBao/Loki backend suite passed in190.984s with exit0 and no resource stop on tested commit3c1e2ad7763aa355cd161467cbade7ee97a7d185. All checked source hashes remained unchanged; peak owned temporary storage was231,477,248bytes and minimum physical free space1,672,310,784bytes. The harness completed its dependency cleanup.

A concurrent PM-only remote advance rejected the initial push. The rebased executable tree and gate inputs are byte-identical to that passing revision; the incremental gate passed. Both commits shipped to origin/main at`c680cd1957db6cc908ad844ccb6f1cb9614f209b` (public-ID regression`d58e9663e`). [Post-ship custody](postship/CUSTODY.json) preserves both full-pass runs, the actual rejected push, rebase path/equivalence proof, final gate and successful-push receipt. Earlier failures and incomplete attempts remain unchanged.
