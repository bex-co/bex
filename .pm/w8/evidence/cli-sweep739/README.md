# CLI sweep 739 — live verification of w8/132–134 after pin `ca30db3a3646` (2026-10-19)

The production pin advanced (`9d7ac8afa` pins `ca30db3a3646`), past the fixes for 132 (`f8008fcb3`), 133 (`f31004f7c`) and 134 (`ca30db3a3`). All three were re-probed live and closed into `done/`:

- **132:** `GET /logs?…&task=tsk-bogus738` / `taskRun` / `sandbox` → 400 `log filter "task" is not supported: bex runs no Render Workflows or sandboxes, so no log belongs to one`; `/logs/values?label=level&task=…` refuses identically; CLI `--task-id` surfaces the same 400. (Subscribe arm not re-probed: websocket; the refusal lives in the shared parser.)
- **133:** `POST /blueprints/generate` malformed and wrong-typed bodies → 400 `invalid blueprint generate body`; `{}` → 400 `select at least one resource to generate a Blueprint from`.
- **134:** `POST /blueprints` nonexistent repo → 400 `code: repo_not_found_or_no_access`; missing manifest path → 400 `code: file_not_found`. Blueprint list unchanged (only the pre-existing `bex`).

w8/135's fix (`410b9ae7b`) is **not** in this pin and remains open for the next one.
