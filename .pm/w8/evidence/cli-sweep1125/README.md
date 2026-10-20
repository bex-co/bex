# CLI sweep 1125 — Content-Type discipline on writes (live, 2026-10-20)

One disposable free web service created and deleted (204, verified 404). No new bug filed.

- PATCH with a JSON body labelled `text/plain` → 400, name unchanged.
- PATCH with `application/json; charset=utf-8` → 200, rename applied on read-back.
- PATCH with no `Content-Type` header → 400, name unchanged.

The write path requires the JSON media type (parameters such as charset accepted) and refuses mislabelled or unlabelled bodies without mutating state.
