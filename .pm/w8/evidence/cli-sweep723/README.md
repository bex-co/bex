# CLI sweep 723 — native-runtime build end-to-end (live, 2026-10-19)

Fixture `qa723-go` (`srv-dbbc6slfsg2s73akoa6g`), native Go runtime from the public repo with explicit build/start commands. Deleted and verified absent (REST 404).

Validation first (client-side, nothing created):

- Native runtime without commands → `--build-command and --start-command are required when runtime is native`.
- Commands without repo/image → `either repo or image is required`.

Build: `queued → build_in_progress → update_in_progress → live` in about 4 minutes; public URL 404 → 503 → 200 (`OK`). Afterwards `bex logs --type build` returned labeled build-log entries and `--type app` showed the request lines from the probes.

No bug filed.
