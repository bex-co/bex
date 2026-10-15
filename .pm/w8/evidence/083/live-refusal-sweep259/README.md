# Managed-data live-tail refusal acceptance — sweep259

2026-10-15 UTC, Bex production, isolated human-QA bex-canary workspace. Source `6255ed8b2ca67c8c0f85fd0eadc19bedbbe62459` contains fix a76d6c187; deployed backend/operator revision is unknown. Published Bex v0.3.2 and unmodified Render2.27.0 / a764810a768202704e7206eb7b87a47211fcd98e now both receive the intended error. Existing [w8/083](../../../done/083.md) is accepted and moved to done; no new issue.

[Actual native commands, stdout/stderr and unchanged HTTP/WebSocket captures](native-tails.json), [twelve diagnostic transport requests and complete replies](transport-refusals.json), [owned create/readiness/history/cleanup ledger](owned-fixtures-cleanup.json), [versions/quota/case summary](metadata.json), [redactions](redactions.json).

One fresh Free native Node App, one Free Key Value (queue/noeviction, journal_snapshot) and one Free PostgreSQL18 (no pooler/read replicas) are created through the real Bex CLI. All become Live/available; the App serves its exact harmless marker over HTTP200. Native mutations are scoped to those exact recorded IDs, absent baseline. No data connection credentials, ACL edits, SQL queries or Redis data writes are used.

For each managed resource, each client runs logs --resources ID --text OWN_MARKER --limit100 --tail --confirm -o json in a fresh process. A transparent local forwarder admits only GET log requests for that exact owned ID and forwards to fixed api.bex.co. Every actual subscription returns HTTP400 JSON with the named family-specific message: managed Postgres/Key Value live log subscription is not supported; use the historical logs query. Native stdout is empty; stderr contains that message and the process exits1. No HTTP101, Upgrade header or silent-success EOF occurs.

| Family   | Client | Native exit | Elapsed | Captured subscription |
| -------- | ------ | ----------- | ------- | --------------------- |
| keyvalue | bex    | 1           | 1.082s  | 400, no upgrade       |
| keyvalue | render | 1           | 0.938s  | 400, no upgrade       |
| postgres | bex    | 1           | 1.15s   | 400, no upgrade       |
| postgres | render | 1           | 0.867s  | 400, no upgrade       |

Twelve additional API-only GET checks cover both managed families, each alone and mixed with the owned App, over WebSocket handshake, SSE and NDJSON request formats. All return the same named HTTP400 with JSON Content-Type, no Upgrade/stream header and no App log line. These are diagnostic transport checks, not additional CLI or browser journeys. A following historical query through each native client exits0 and returns exactly five records, correctly labeled for its managed resource; historical collection remains available on these fixtures. This small query does not resolve the independent Postgres burst/sampling issue in w8/m65.

After the refusals, each client starts a supported App live tail through the same capture mechanism. The recorded WebSocket101 precedes the single HTTP emit request. Each receives exactly one own nonce line with the correct resource label. The harness then sends SIGINT and reaps the client (exit-2 is this explicit stop, not a product error). Both relays and copied configs are removed. This verifies a supported tail still works after the refusal checks; it does not certify all subscription-accounting states, App/build classes or stream formats.

The existing [focused real-handler regression](source-test-events.jsonl), TestSubscribeRefusesManagedLiveTailsBeforeStreaming, passes on current source. It covers both families, single/mixed requests and all three formats; no product tests were added and the full backend suite was not rerun. Source review confirms the one shared refusal runs after normal admission and before transport selection, with deferred release, and the direct follower still uses the same helper. No extra abstraction or duplicate refusal was introduced by QA.

All three native deletes exit0. Managed resource details return404 and native family lists are empty; App detail and former public URL return404, with its one terminal deploy child enumerated before parent deletion. All six baseline services remain and final quota is services6/terminating0, Postgres0/0, Key Value0/0. No direct database endpoint teardown or live Kubernetes descendants were inspected. Existing auth remains isolated for the continuing hunt. No product change, commit/push, deployment or external message.

Reproduction sources: [resource driver](fixture-driver.py.txt), [native capture/positive-tail driver](native-tail-driver.py.txt), [bounded exact-owned forwarder](forwarder.go.txt), [diagnostic transport driver](api-transport-driver.py.txt). The forwarder is reused from the original finding and admits only the new ID passed to it; it never targets the old deleted fixture.
