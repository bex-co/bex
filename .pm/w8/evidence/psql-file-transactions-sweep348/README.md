# Native psql file forwarding and whole-script transactions — sweep348

[Results](results.json) · [complete native commands, SQL bodies/results, ordinary API receipts and cleanup](native-SQL-API-cleanup.json) · [finite journey](finite-file-transaction-journey.py.txt) · [isolated client setup](isolated-native-helpers.py.txt) · [cached teardown diagnostic](cached-SQL-teardown-diagnostic.py.txt).

Source c84c348b0e4a2248ee0f80a09cc15165275e14d7 equals refreshed main; serving revision unobserved. Published Bex0.3.2/a6350da0 creates one fresh explicit Free Postgres18, no pool/replica/HA, with authored custom database/user in bex-canary. Native exit0, returned exact ID/name/workspace/Free and empty pre-inventory prove ownership. Native mutation HTTP wire is not independently captured. Safe readiness reads retain the normal creating→available transition; no SQL writes occur while provisioning. A native read-only SQL control verifies the exact created database/user, server18 and UTF8 before any script mutation.

## Actual native file and transaction behavior

Both Bex and unchanged Render2.27.0/a764810a7682 run the documented --command mode with additional psql flags after --. Each invocation forwards --single-transaction, ON_ERROR_STOP=1 and **two --file arguments whose local paths contain spaces**. The first command creates a uniquely named own table and inserts its first row; file1 uses a psql variable and inserts another row.

- For each client, file2 deliberately divides by zero. Both native parents terminate nonzero with the actual script error. A fresh query through the other client proves to_regclass is null: the original table creation and prior inserts are rolled back, not partially retained. The native parent wraps the child error, so no claim is made that its exit equals psql's raw3.
- For each client, the successful file2 commits three additional rows and prints an ordered JSON aggregate. The five rows exactly preserve the command marker, psql variable, multiline Japanese/accented/emoji text, empty string and SQL NULL. Both fresh cross-client reads return that same array. Successful native calls exit0; no timeout is counted as a pass.

Nine native SQL comparisons pass: one connection/identity control, four file executions and four independent rollback/data checks. The eight actual file bodies are retained beside the receipts and all temporary execution copies are removed. Their command/file action ordering follows [PostgreSQL's psql contract](https://www.postgresql.org/docs/18/app-psql.html): combined command/file actions share the single transaction and an ON_ERROR_STOP error rolls it back. No authenticated Render hosting account is used.

The pinned cmd/psql.go requires --command for noninteractive output and forwards post-dash arguments. pkg/tui/views/psql.go:45–90 obtains this owned database connection, constructs the first -c action, appends each extra argument unchanged and runs the real local psql; JSON returns the captured output field. These actual end-to-end checks exercise that path rather than tests that only marshal the result type. No product/Go test code is edited and no full backend suite is claimed.

## Setup and cleanup

The unchanged Render client receives this owned database's returned CA through a0600 temporary PGSSLROOTCERT file for each SQL invocation, avoiding the known external-client setup gap already recorded in w8/079. Only a certificate hash/status is retained; all connection-info credential fields are omitted before ledger and no personal root-cert/config is changed. Bex uses its published connection setup.

An ordinary cached psql diagnostic succeeds before delete and no longer executes SELECT1 after deletion; those two probes are API-only data-client evidence, not Bex/Render native coverage. Private cached credentials and its CA are removed. Native Bex delete exits0. Own detail404, empty native PG list, original6 services and PostgreSQL quota0/terminating0 prove cleanup. Physical Kubernetes/PVC inventories and direct/private endpoints remain unobserved. The isolated QA login remains for the continuing authorized loop.

This is a passing functional terminal journey with no new confirmed bug. It does not certify concurrent transactions, interrupted imports, large files, paid pools/replicas, CSV COPY, interactive stdin or browser behavior. Baseline configurations are projected out and QA credentials masked. Uncommitted.
