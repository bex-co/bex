# Ctrl+C reports cancellation while the external PostgreSQL query keeps running

Why: Customers need to stop their active SQL query while retaining their interactive session; a successful dispatch message currently leaves the work running to completion.

**Severity:** major. **Attribution:** external PostgreSQL ingress/runtime. **Date:**2026-10-16. **API:**https://api.bex.co/v1/. **Workspace:**tea-daif693dqjvc73e7as3g/bex-canary. Existing isolated QA OAuth credential; new disposable Free PG18, no pooler/replicas. Published Bex0.3.2/a6350da0d6e451e063143432e06c548ba7b5851b and unchanged Render2.27.0/a764810a768202704e7206eb7b87a47211fcd98e; source7079d02b69a27026ab0b3dce618567182179fcc7. Actual external psql/libpq18.6. Serving revision is unknown. [Complete evidence](evidence/cli-sweep365/README.md).

## Reproduction and exact outcome

Create a uniquely named explicitly Free PostgreSQL18 through the normal native command, wait for available and an actual SELECT1, then open interactive psql on its recorded ID. Render needs the existing per-database CA preparation; Bex supplies it through its normal launcher path.

```sh
bex postgres create --name "$FRESH_NAME" --plan free --version 18 --region frankfurt --connection-pool none --confirm -o json
bex psql "$OWNED_PG_ID" --confirm -o interactive -- -X -n -q -t -A -P pager=off -v ON_ERROR_STOP=off
# In this session:
# BEGIN; INSERT INTO an_own_scratch_table VALUES ('own marker');
# SELECT pg_sleep(12);
# Press Ctrl+C after an independent same-role native query confirms this PID is active/PgSleep.
# After the next prompt: \echo :SQLSTATE
# ROLLBACK; SELECT pg_backend_pid(); then \q.
```

The finite recorded driver uses unique prompts/application_name and statement_timeout45s and exercises both client binaries in separate config copies. A second same-role native psql command reads only the exact own PID/application_name in pg_stat_activity before and after Ctrl+C.

| Client | psql dispatch | Still sleeping after Ctrl+C | Query duration | SQLSTATE | Connection reuse / natural parent exit |
| --- | --- | --- | --- | --- | --- |
| Bex0.3.2 | Cancel request sent | yes | 12.631s | 00000 | same PID, rollback rows0 /0 |
| Render2.27.0 | Cancel request sent | yes | 12.943s | 00000 | same PID, rollback rows0 /0 |

Expected is the actual statement returning user cancellation/57014 promptly and the session remaining usable; the success of the dispatch alone is insufficient. A documented modern libpq cancellation on this exact endpoint cancels the active query in0.701s/57014 and reuses the same connection. The same psql18.6 and terminal cancel against a private local PG18.6 in0.004s/57014. This distinguishes an ingress compatibility gap from a broken SIGINT harness or unavailable PostgreSQL cancellation. Both settled native runs use40x140 terminals with ISIG enabled. Earlier forced-cleanup panels are explicitly excluded.

Native create/delete/psql entrypoints and SQL observations are fully recorded; the create/connection-info transport was not captured as a wire trace. Diagnostic GET requests and complete responses are retained after sanitization, while connection-info credentials/URLs are excluded. No crafted production packet, cancellation-key extraction, privileged SQL or unrelated session is used.

## Producer, consumer and failing boundary

1. The pinned upstream cmd/psql.go interactive path loads connection-info; pkg/tui/views/psql.go:182 loadDataPSQL invokes external psql with the returned externalConnectionString and forwarded args. pkg/tui/exec.go:36 uses Tea.ExecProcess. The same unchanged Render client and direct external psql reproduce, so this is not a launcher request rewrite problem.
2. Exact external-tool source [PostgreSQL REL_18_6 fe_utils/cancel.c](https://github.com/postgres/postgres/blob/REL_18_6/src/fe_utils/cancel.c#L153): the SIGINT handler uses the legacy PQcancel. [fe-cancel.c](https://github.com/postgres/postgres/blob/REL_18_6/src/interfaces/libpq/fe-cancel.c#L548) connects a new socket to the cached remote address and sends the cancellation message without a TLS hostname. Its successful dispatch is consistent with psql's message even though the proxy closes the new socket.
3. lego/operator/cmd/pg-sni-proxy/main.go:427 reads the first8 bytes. Lines438–441 accept only SSLRequest or direct TLS and close an ordinary CancelRequest before route lookup. A private overlay invoking this actual handler proves the close for valid16-byte PG3.0 and44-byte PG3.2 synthetic messages on own local sockets. No hypothetical malformed-input finding is involved.
4. main.go:172–186 indexes primary, optional pooler and replica aliases. Lines470–527 resolve SNI and replay the SSL preamble, then relay TLS end to end to CNPG. It cannot observe BackendKeyData inside that encrypted stream or route a hostname-free message. Accepting the message header alone is therefore insufficient.
5. [ADR009](../../../../docs/ADR009-postgresql-management.md) specifies one wildcard/shared5432 endpoint, no per-database LB, and TLS passthrough. The proxy is a DaemonSet behind a shared LB (config/pg-sni-proxy/daemonset.yaml, config/prod/pg-sni-proxy-lb.yaml); cancellation can reach a different instance. database_controller.go:548–553 adds public aliases to the CNPG server certificate; backend/internal/postgres/service.go:822–935 returns verify-full URLs and that database's CA for primary/pooler/replicas. These connection contracts and their consumers must stay coherent.

## Concrete repair and remaining design decision

Implement ordinary cancellation for the shared public PostgreSQL endpoint. The workable shared-endpoint design is a protocol-aware frontend that observes the authenticated startup BackendKeyData, substitutes a proxy cancellation identity, and retains a connection-lifetime mapping to the exact upstream connection's backend PID/key/destination. Later legacy or TLS cancellation resolves that identity and forwards to that exact backend, then the original SQL stream carries57014. Keep current modern TLS cancellation working. SQL row data can remain streaming; this is not a SQL parser or a new user API.

This requires explicitly revising ADR009's passthrough decision: with unchanged existing psql and the same shared destination, the current encrypted relay cannot manufacture or learn that routing identity. Task1 must select and document the exact mediation/owner-discovery mechanism before implementation. A concrete candidate is per-database CNPG-issued frontend serving material, separately verified upstream TLS, and a synthetic identity containing a registered proxy-owner identifier plus a per-connection lookup key. A cancel arriving at another proxy forwards internally to the owner; connection close/proxy loss expires the entry. Owner allocation, restart handling and framing must be specified and tested rather than assuming a process-local map suffices. An equivalent shared registry is an implementation alternative, not an already-selected dependency.

Trace the exact serving Secret and rotation contract through CNPG instead of hardcoding guessed Secret names. Current proxy RBAC only reads Database CRs; any new certificate/owner projection must be explicitly owned by the implementation and included in its local integration tests. Preserve current verify-full URLs and downloadable CA consistency, direct TLS and SSLRequest, normal auth including supported channel binding, original endpoint identity, bounded relay behavior, byte accounting and transaction/session continuity. Revise producer and consumer together if the reviewed design changes serving material. No paid per-database infrastructure, source-IP route guessing, broadcast-to-all backends, client fork or forced external-tool replacement satisfies this scoped fix.

## Blast radius, regression and limits

One shared PostgreSQL ingress handler; three alias families: primary, optional PgBouncer, and named replicas (currently all replica aliases select the shared -ro service). The cancellation mapping must pin the actual selected upstream server for that connection, including a service with multiple pods; reconnecting to a random replica is insufficient. Key Value's separate Redis proxy has no PostgreSQL cancel semantics. REST/GraphQL/MCP/dashboard connection-info consumers must continue to publish the same usable URL/trust contract. Live tested only an unpooled Free primary on PG18; pooler, replicas, multiple ingress nodes, failover, other PG/psql versions, direct TLS and channel binding are unverified acceptance neighbors.

Acceptance: fresh Bex and unchanged pinned Render sessions execute own pg_sleep(12), independently establish active own PID, press real Ctrl+C, observe57014 within3s, prove the active query ends before12s, ROLLBACK and reuse the same connection/PID, then quit naturally0. Retain the modern cancellation and local psql controls. Local tests must cover two owned backend routes, same source address, cross-proxy delivery, startup key framing for both protocol forms, close/restart cleanup, primary/pooler/replica destination pinning and certificate rotation. No production foreign-route tests are needed. The overlay currently passes by characterizing the bug; it is not proof of a repaired ingress.

Open/done/blocked searches for CancelRequest, pg_cancel_backend, query cancellation and psql Ctrl+C found no existing implementation item. Known [w8/085](../085.md) is a blank interactive error panel after child failure; [w8/079](../079.md) loses successful noninteractive warnings. Neither owns a running query ignoring Ctrl+C. Targeted proxy history has no pending cancellation fix; all relevant producer/consumer code remains as traced at source HEAD. No deployed revision is asserted and no bug is inferred merely from an old release.

Native delete0/detail404/native PG list empty, cached connection before success/after failure2, zero remaining own sessions/rows, all owned local processes/config copies/history/cached credentials/CA removed. Original six services/one Blueprint remain, PG/KV usage and all terminating counts0. No paid fixture, unrelated mutation, product fix or new commit. Implementation estimate435m across eight tasks, including standing parity/simplify/coverage/closeout; scheduling only.
