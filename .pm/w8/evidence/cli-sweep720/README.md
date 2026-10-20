# CLI sweep 720 — static-site headers and routes, enforced (live, 2026-10-19)

On sweep 719's static fixture, before deleting it (verified absent, REST 404).

- `PUT /headers [{"path":"/*","name":"X-QA720","value":"yes"}]` → 200; within ~20s the public response carried `x-qa720: yes`.
- `PUT /routes` rewrite `/qa720-old → /index.html` → 200; the path served 200. Adding a `redirect` route alongside (replace-all body with both rules) → public request answered `301` to `/index.html`. Rules read back with minted ids and priorities 0/1.
- Refusals: route `type:"bogus"` → 400 `invalid request body at /0/type`; missing destination → 400 at `/0/destination`; header name `"Bad Header"` → 400 `headers[0].name "Bad Header" is not a valid HTTP header name`.

The static-only surface refused to web services (sweeps 672/718) works fully on a real static site. No bug filed.
