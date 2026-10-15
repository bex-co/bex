# PostgreSQL committed data across two native suspend/resume cycles — sweep320

[Native create, SQL, lifecycle commands, readbacks and cleanup](native-journey.json) · [finite guarded driver](journey-driver.py.txt) · [six native help reads](native-help.json) · [source results](source-results.json).

Published Bex0.3.2/a6350da0 creates one fresh, explicit Free PostgreSQL18 in bex-canary, Frankfurt region, connection pool none, no replicas or HA. Pre-create native Postgres inventory is empty, the original six services are intact and the Free database slot is available. Native exit0 plus returned ID/name/workspace/free/version establish ownership. Local source is c11f862351e18a6c525db9ea55d7673d18237091; serving revision is unobserved. Provisioning is observed as creating before available. No paid resource, cron job or baseline mutation is involved.

## Results

The published Bex psql command opens real local psql18.6 with -X, -q, -t, -A and ON_ERROR_STOP=1 through its supported -- passthrough. Inherited PG overrides are removed; -X avoids personal psql startup commands. No personal root certificate exists. The launcher provisions the owned database CA for each invocation; no connection-info response, URI or password is retained by this driver. Unmodified Render is used for lifecycle commands only; its psql trust path is not certified here.

A committed table contains four identity-keyed rows covering exact UTF-8/emoji, embedded newline/tab, NULLs, fixed-scale positive/negative numeric values, JSON objects/arrays and binary values including zero/high bytes. The first complete snapshot matches the independently constructed expected rows, sequence last_value4/is_called:true and server_version_num180006. Exact name selection is used for the initial psql read.

Cycle1 uses published Bex suspend/resume by immutable ID. Both commands exit0. A native read reports suspended and a fresh native psql connection exits1 with the server's shutting-down refusal. After resume converges to available, every row and sequence field matches; postmaster start time changes. A second committed transaction increments the four row counters and inserts identity row5. A separate transaction changes all text and deletes the Unicode row, then rolls back.

Cycle2 uses unchanged Render2.27.0/a764810a7682 suspend/resume by the unique owned name. Native inventory immediately beforehand proves the name resolves to only this ID. Both commands exit0, readback reports suspended, and a fresh native psql connection exits1 with unexpected EOF during TLS while the database is stopped. After available, the final psql snapshot by name matches the four incremented rows plus the exact fifth row, retains the rolled-back Unicode row and original texts, and reports sequence last_value5/is_called:true. Postmaster start time changes again.

The three observed start times are 2026-10-15 22:03:16.755416+00, 2026-10-15 22:05:47.523903+00 and 2026-10-15 22:08:21.319044+00. All three complete data comparisons and both process-change checks pass. There is no new confirmed defect. This extends the single-row native restart control in [w8/084](../../blocked/084.md), without recertifying that upstream failure-status fix or claiming its gate is complete. The selected backend action-capability and operator lifecycle-annotation tests both pass; no product implementation or new test is added.

## Scope and cleanup

One owned database is shared across the two lifecycle clients. This proves small graceful suspend/resume retention, local native psql execution by ID/name, a later committed identity value and rollback isolation through the next process. It does not prove abrupt crash/power-loss recovery, backup/restore, pooling, replicas/HA, paid plans, concurrent transactions, direct raw-Render SQL, browser parity or a deployed SHA. The exact process-stop instant and public socket teardown after deletion are not measured.

Before delete, the exact owned detail confirms no replicas and pool none, and the run ledger contains no dependent fixture. Native delete exits0. Detail returns404, native Postgres list is empty, the original six service IDs remain and Postgres quota returns0 used/0 terminating. The isolated copied Render config is removed after every Render command. The launcher owns its per-command temporary CA cleanup; this driver never saves a CA/connection credential file. The ongoing QA session remains for the requested loop. This evidence stays uncommitted pending another ship instruction.
