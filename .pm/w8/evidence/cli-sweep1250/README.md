# CLI sweep 1250 — static-site request logs, tail and metrics (live, 2026-10-20)

One free no-build static site (`srv-dbbja9hq56vc73bn8eo0`, `mdn/beginner-html-site`) created with `bex services create --type static_site`, live in 33 s, deleted (GET 404). No new bug filed.

- `bex logs -r <static> --tail --type request -o json` streamed the three requests live, including the marker path. History reads with `--type request`, `--path /<marker>-missing` and `--text <marker>` return exactly those lines. `--status-code 404` is empty because the marker path answered 200 (see below). `GET /v1/metrics/http-requests` counts 3.
- An extension-less missing path (`/zzz-missing`, `/a/b/c`, `/images/nope`) answers 200 with `index.html`; a missing path with an extension (`/zzz.png`, `/zzz-missing.html`) answers a plain-text 404. This is ADR029's documented “Default SPA fallback for extension-less misses (deliberate Render divergence)”; Render requires an explicit `/*` → `/index.html` rewrite. Not filed.
- ADR029:26 states the cache-key assumption that w8/146 breaks (“keyed by revision prefix, so it is never stale and is never purged”). That line was cited in w8/146.
