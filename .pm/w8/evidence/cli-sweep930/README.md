# CLI sweep 930 — wrong-resource subpaths 404 cleanly (live, 2026-10-20)

Disposable free web service created, probed, deleted (204, then GET 404). Pin unchanged; 31 local paths pending. No new bug filed.

- `POST /services/{id}/webhooks`, `GET /services/{id}/ssh`, and `GET /services/{id}/connection-info` → 404 (webhooks are workspace-level, sweep 838; SSH is the gateway surface and connection-info is datastore-level, sweep 824) — invented subpaths do not fall through to a parent handler that might misinterpret them.
