# m87 design — PostgreSQL cancellation through the shared ingress

**Status:** proposed 2026-10-16, awaiting the user's decision on the trust-boundary change below. ADR009 is amended only once it is accepted.

## Why passthrough cannot carry a legacy cancel

- psql's SIGINT path calls `PQcancel`. That call opens a new socket to the cached address and sends a bare 16-byte (PG 3.0) or variable-length (PG 3.2) `CancelRequest`, with no TLS and no SNI.
- The only routing data in that message is the backend PID and secret key. The backend sent those in `BackendKeyData` inside the TLS stream, which `pg-sni-proxy` (`lego/operator/cmd/pg-sni-proxy/main.go:392–527`) relays opaquely.
- With passthrough the proxy never sees the key, so no amount of header handling can route the cancel. The only routing data available would be the client's source IP, and the finding already rejects routing on it: shared NAT fans the cancel out across tenants, and each wrong-key cancel writes a LOG line naming a PID into another tenant's log.

**Conclusion:** the proxy must terminate client TLS to observe `BackendKeyData`. Every design that works without that is a guess.

## Proposed mechanism

1. **Terminate per database with that database's own CNPG serving pair.**
   - Read the Secret names from the CNPG Cluster's `status.certificates.serverTLSSecret` and `serverCASecret`. They are not guessed: the backend already reads `<db>-ca`, at `backend/internal/postgres/service.go:916`.
   - Serve the cert selected by SNI in `GetConfigForClient`. That is the same leaf CNPG serves, which already carries every public alias (`database_controller.go:549–554`).
   - Clients' `verify-full` URLs and the downloadable CA therefore stay unchanged.
   - SCRAM `tls-server-end-point` channel binding is a hash of the server certificate. Because the client and the backend see the same certificate, `SCRAM-SHA-256-PLUS` keeps working.
   - The upstream leg is a separate TLS connection verified against `serverCASecret`.
   - Before answering the client, the proxy compares the backend's presented leaf with the one it serves. On a rotation skew it refreshes the Secret once, then fails closed. It never strips PLUS.
2. **Observe the key; don't substitute it.**
   - Parse only startup/auth framing, up to the first `ReadyForQuery`. Record the `BackendKeyData` (PID plus 4-byte or PG 3.2 variable key) against the exact upstream pod address, then splice as today. The meter is unchanged.
   - The client keeps the real PID and key, so `PQbackendPID` and the self-filtering of `NotificationResponse` stay correct.
   - No SQL parsing, and no rewriting of the server stream.
3. **Pin the exact backend pod.**
   - Dial a ready endpoint from the route's EndpointSlice instead of the Service ClusterIP, so the cancel reaches the same `-ro` replica or PgBouncer instance.
   - New RBAC: list/watch on `endpointslices` and the two CNPG Secrets.
   - Primary failover kills the old connection anyway.
4. **Deliver across proxy instances.**
   - A cancel's TCP connection can land on any DaemonSet pod behind the Hetzner load balancer.
   - The receiving pod checks its own table. On a miss, it forwards the raw message to its peers (a headless Service over the proxy pods) on an internal port, restricted by NetworkPolicy to proxy pods.
   - Only the pod holding `(pid, key)` sends the real `CancelRequest` to the pinned pod.
   - Entries die with the connection. A proxy restart drops its table, together with every connection it carried.
   - Fan-out is bounded by peer count and rate-limited per source.
   - Direct-TLS and SSLRequest cancels resolve the same way. Modern libpq cancellation already works through SNI and is unchanged.

## What the user is deciding

The internet-facing `pg-sni-proxy` would change in four ways:

- It would hold **every tenant database's server private key**, which today only CNPG pods hold.
- It would see **plaintext query traffic in flight**. SCRAM keeps passwords off the wire either way.
- It would pay double TLS crypto on the 100m-CPU shared proxy. Resources need sizing.
- ADR009's documented "TLS passthrough" contract would be revised, and the ADR056/ADR067 proxy review assumptions would need a fresh review.

The alternative is to keep passthrough and document that only modern libpq (`PQcancelCreate`, libpq 17+) cancellation is supported through the shared endpoint. psql's own Ctrl+C would remain non-functional.

## Estimate if accepted

t002–t008 as filed (~6.5h), plus a security-review pass on the new key custody.
