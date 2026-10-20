# CLI sweep 1235 — fresh repeat of the Node first deploy: 3m43s, container up in 7 s (live, 2026-10-20)

Same `bex services create … --repo https://github.com/render-examples/express-hello-world --runtime node …` as sweep 1233, with a new nonce (`srv-dbbgo13581vs73bjhigg`). Deleted with `bex services delete --confirm` (GET 404). No bug filed.

- `created` → `queued` (49 s) → `build_in_progress` (145 s) → `build_ended` 06:40:15Z. The instance appeared at 06:40:11Z, the app logged “listening” at 06:40:18Z, and the deploy went `live` at 223 s with the URL returning 200. Sweep 1233's 13-minute container start did not recur.
