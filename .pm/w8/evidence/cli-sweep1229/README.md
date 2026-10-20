# CLI sweep 1229 — zero dropped requests across a CLI deploy and restart (live, 2026-10-20)

One disposable free web service (`qa-20261020-86b264-zdt`, `srv-dbbg5pj581vs73bjhhk0`, `docker.io/ealen/echo-server:0.9.2`) created with `bex services create`, deleted with `bex services delete --confirm` (GET 404 verified). HEAD checkout build `94533d795`, workspace `bex-canary`. No new bug filed.

- Four clients requested the public URL continuously (~4 req/s) for ~3.5 minutes spanning `bex deploys create <id> --wait` (exit 0, `live` in 21.5 s) and `bex restart <id>` (exit 0, “restarted successfully”). **884/884 requests answered 200** — no 5xx, resets or timeouts at either rollout.
- `bex deploys create --wait` blocks until the deploy is `live` and prints the final deploy object; the restart opens its own deploy row (`trigger: api`), as ADR018's Restart row documents. Events show matching `deploy_started`/`deploy_ended` pairs for all three releases, and `GET …/instances` lists exactly the restart's new instance afterwards.
