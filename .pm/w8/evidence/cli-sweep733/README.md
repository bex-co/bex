# CLI sweep 733 — `deploys create` commit guard and `--wait` exit codes (live, 2026-10-19)

Disposable free image service `qa733-web` (`srv-dbbcf544kujs7397oah0`). Deleted and verified absent (REST 404).

- `deploys create --commit <sha>` on an image-backed service → 400 `commitId is not supported for image-backed services — deploy an image tag or digest with imageUrl`.
- `deploys create --image <bad-tag> --wait` waited for the terminal state and exited **1** with the `update_failed` deploy object; the same wait on the good image exited **0**. (An initial reading of exit 0 was the harness's own pipeline artifact — `$?` measured `tail`, not the CLI — and was discarded after re-measurement without a pipe.)

The documented `--wait` contract ("exit non-zero if deploy fails") holds. No bug filed.
