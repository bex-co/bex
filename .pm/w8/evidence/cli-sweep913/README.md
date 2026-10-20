# CLI sweep 913 — public URL lifecycle end-to-end (live, 2026-10-20)

Disposable free image-backed web service created, fetched publicly, deleted (204). No new bug filed.

- `https://qa913-web.onbex.co` answered 404 on the first probe (edge not yet attached) and 200 with the hello-app body (`Hello, world! Version: 1.0.0`, tenant-scoped hostname) ~10 s later — deploy-to-public-serve works at free tier with no manual wake step needed here.
- After DELETE, the same URL → 404: the public route dies with the service (no dangling edge serving a deleted workload).
