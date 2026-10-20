# CLI sweep 1257 — bad branch / root-directory updates fail the deploy cleanly (live, 2026-10-20)

One free native Node web service (`render-examples/express-hello-world`, `srv-…` recorded in the private ledger) created with `bex services create --repo … --runtime node`; first deploy live in 189 s. Deleted (GET 404). No new bug filed.

- `bex services update <id> --branch does-not-exist-qa` → exit 0, saved without deploying (latest deploy unchanged), as Render's settings update does. `bex deploys create <id> --wait` → exit 1 in 12 s with deploy `build_failed`, `failureReason: build failed in the clone step: fatal: couldn't find remote ref does-not-exist-qa`. The previous release kept serving 200.
- `--branch main` then `--root-directory nope-dir` → the next `--wait` deploy fails in 19 s: `the root directory "nope-dir" does not exist in the repository at "<sha>"`, and the service still serves.
