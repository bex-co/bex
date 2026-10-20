# CLI sweep 1081 — CLI services delete (live, 2026-10-20)

One disposable free web service created via REST and deleted via the CLI. No new bug filed.

- `bex services delete {id} --confirm` prints the deleted service's summary (name, id, type, owner, dashboard URL) and the REST GET immediately answers 404 — CLI deletion lands in the same store.
