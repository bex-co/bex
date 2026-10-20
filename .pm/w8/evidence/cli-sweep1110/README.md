# CLI sweep 1110 — no conditional-request support on the API (live, 2026-10-20)

Read-only probes on the baseline `hello-go` service; the sweep-1109 name census is empty. No new bug filed.

- Service GETs carry no `ETag`/`Last-Modified`, and `If-None-Match: *` still answers 200 with the full body. The API has no conditional-request contract (consistent with the header observations in sweep 1010); recorded, not filed.
