# CLI sweep 1011 — malformed request bodies (live, 2026-10-20)

One disposable free web service created and deleted (204). No new bug filed.

- A syntactically invalid JSON body and a JSON body sent as `text/plain` both get the same coded 400 `request body does not match the Render API schema` — no 500, no partial write (the rename in the second probe did not take).
