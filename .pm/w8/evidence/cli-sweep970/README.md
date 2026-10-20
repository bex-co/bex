# CLI sweep 970 — custom domain lifecycle (live, 2026-10-20)

Two disposable free web services created; both deleted (204s; GETs 404). No new bug filed.

- Add `qa970.example.com` → 201 pending, with the CNAME target and the ownership TXT challenge (`_bex-challenge`) issued in the response. Invalid and empty names → 400.
- The first delete attempt's 400 was a harness error (it passed the wrapped list envelope's missing id, i.e. `None`); deleting by the domain name → 204 and the list empties. A second delete of the same name also → 204 (idempotent delete; recorded).
- `POST …/custom-domains/{name}/verify` before any DNS exists → coded 409 `DOMAIN_OWNERSHIP_PENDING` spelling out the exact TXT host/name/zone expected — a refusal that tells the caller precisely what to publish.
- Cross-service/own-host domain conflict behavior was already covered in sweep 842 and is not repeated here.
