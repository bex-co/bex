# Native Redis pipe import and exact stored bytes — sweep349

[Results](results.json) · [complete native commands/PTY output, payload hex, stored-byte proofs, ordinary API setup and cleanup](native-pipe-data-API-cleanup.json) · [local byte preflight](local-PTY-byte-preflight.json).

Source c84c348b0e4a2248ee0f80a09cc15165275e14d7 equals refreshed main; serving revision unobserved. Published Bex0.3.2/a6350da0 creates one fresh explicit Free/off/noeviction Key Value in bex-canary. Native exit0, exact returned ID/name/workspace/Free and empty pre-inventory prove ownership. Normal API-only public:true setup converges through config_restart to available; both Bex and unchanged Render2.27.0/a764810a7682 then execute actual native redis-cli PING/PONG, exit0. No authenticated Render account is used.

## Two native imports, independent reads

Each client runs the supported interactive kv-cli passthrough with redis-cli --pipe --pipe-timeout10. Each imports50 distinct own scratch keys, in a client-specific namespace. Ten values per batch are empty;40 contain newline, Unicode, CRLF and NUL bytes. The small batches are only a few KiB. Input is raw RESP with byte lengths and a real EOF at the terminal handoff, not shell-interpolated text.

Both native parents terminate0 without reaching the harness bound. Each reports **errors0/replies50** and completion of the last response. Through a fresh invocation of the opposite client, a read-only Lua GET/hash check verifies all50 values and their exact aggregate byte count, including presence of empty strings. All100 values match. Payload hex, full non-secret native command/output/exit/timing and expected hashes are retained; mutation HTTP wire is not independently captured.

The [official Redis bulk-loading contract](https://redis.io/docs/latest/develop/clients/patterns/bulk-loading/) describes raw protocol input and the final reply accounting. This journey uses that normal import behavior; it is not a throughput benchmark or a large import test.

## PTY qualification

The pinned kv-cli supports interactive mode only. A real bounded PTY supplies that documented prerequisite. A local read-all process first proves the entire generated input hash is preserved, including CRLF/NUL/Unicode/empty frames. Input translations/echo are disabled while canonical EOF is retained. The runner waits for this run's actual redis-cli/valkey-cli descendant by process name before sending data; it reads no process arguments or credentials. Nonblocking input and a35-second bound prevent a stuck writer. Each actual record proves expected payload hash, bytes+EOF written and exact owned process-group cleanup. No unbounded background mutation runner is used. Stdout/stderr are merged by the PTY and are not falsely presented as separate streams.

This verifies whole-pipe completion beyond the earlier single-value stdin control; the passing count is backed by actual stored bytes, not PONG alone. No product/test code is edited and no module suite is claimed. [Payload generator](authored-RESP-payload.py.txt), [bounded PTY](bounded-byte-preserving-PTY.py.txt), [finite native journey](finite-native-pipe-journey.py.txt), [isolated client helpers](isolated-client-helpers.py.txt).

## Cleanup and limits

An ordinary cached TLS PING succeeds before deletion and no longer receives PONG afterwards; these two probes are API-only data-client diagnostics. Native Bex deletes the exact standalone owned cache, exit0. Detail404, empty native Key Value inventory, original6 services and quota Key Value0/terminating0 prove cleanup. Cached credentials and all copied Render config files are removed. The isolated QA login remains for the authorized continuing loop. Baseline configuration is projected out and QA credentials/connection URIs masked.

No new confirmed defect. Large streams, connection interruption/retry, error-bearing pipelines, persistence/restart durability, multiple database indexes, other client versions and physical cluster artifact teardown remain unexercised. [Cached endpoint diagnostic](cached-TLS-teardown-diagnostic.py.txt). Uncommitted.
