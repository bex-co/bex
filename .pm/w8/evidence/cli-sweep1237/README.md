# CLI sweep 1237 — per-deploy image override follows Render's tag-only contract (live, 2026-10-20)

One free web service (`qa-20261020-*-imgovr`, `srv-dbbgr9h9a20c7385rb80`) created from `gcr.io/google-samples/hello-app:1.0`, deleted with `bex services delete --confirm` (GET 404). No new bug filed.

- `bex deploys create <id> --image gcr.io/google-samples/hello-app:2.0 --wait` → exit 0. The URL serves `Version: 2.0.0` and the deploy row's `image.ref` is `:2.0`, while the service's saved `imagePath` stays `:1.0` and `undeployedChanges` reads true. This is Render's “for future deploys, your service continues to use the tag or digest in its settings”.
- `bex restart` keeps serving 2.0.0 (Render: restart runs the running deploy). A plain `bex deploys create <id> --wait` returns to 1.0.0 and clears `undeployedChanges`.
- `--image docker.io/library/nginx:1.27` (different repository) → exit 1, `400 … imageUrl must use the service's configured image gcr.io/google-samples/hello-app (got docker.io/library/nginx); only the tag or digest can change per deploy …`. Render's “all components of imgURL besides the tag or digest must match” rule, with an actionable message. Nothing changed.
- Observation (followed up in sweep 1238): the override deploy took 452 s. Its new container logged “Server listening” 7 minutes after the deploy started. The restart (same tag, now cached) started in 11 s, and the plain deploy (`:1.0`, cached) in 2 s.
