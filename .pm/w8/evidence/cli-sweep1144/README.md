# CLI sweep 1144 — type-scoped subroutes refuse on a web service (live, 2026-10-20)

One disposable free web service created and deleted (204, verified 404). No new bug filed.

- `GET /services/{id}/headers` on a web service → 400: header rules are a static-site surface (the w8/137 domain) and refuse on the wrong service type rather than returning an empty list.
- `GET`/`POST /services/{id}/disks` → 404: no nested disks collection exists on this route shape.

Subroutes are scoped to the service types that own them; cross-type probes fail closed.
