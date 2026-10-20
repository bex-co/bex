# CLI sweep 803 — events feed + logs on a fresh fixture (live, 2026-10-20)

Disposable free web service created, deployed, inspected, deleted (204, then GET 404). No new bug filed.

- `GET /services/{id}/events?limit=5` → 200 with the create deploy's `deploy_ended` (`deployStatus: succeeded`, image, started/finished) and cursor envelopes.
- Bogus `type=` filter → 400 with the full allowed-type enumeration in the message (named refusal).
- `GET /logs?resource={id}` → 200 with the app's real first log line (`Server listening on port 3000`), `hasMore:false` and time-window cursors.
- CLI note: `bex logs` takes no `--service` flag (usage error printed) — the pinned CLI's positional/flag shape, not a server issue; REST parity above is the contract checked here.
