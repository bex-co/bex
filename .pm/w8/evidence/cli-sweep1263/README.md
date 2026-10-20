# CLI sweep 1263 — Render's FastAPI quickstart through the CLI (live, 2026-10-20)

`render-examples/fastapi` (its `render.yaml`: runtime python, `pip install -r requirements.txt`, `uvicorn main:app --host 0.0.0.0 --port $PORT`). The fixture was deleted (GET 404). No new bug filed.

- Without `--build-command`, the pinned CLI refuses client-side (`--build-command and --start-command are required when runtime is native`), so the README's “Render will detect Python and use pip” dashboard path is not reachable through the CLI on Render either. Upstream behaviour, not filed.
- With the README's commands: `bex services create --runtime python --build-command 'pip install -r requirements.txt' --start-command 'uvicorn main:app --host 0.0.0.0 --port $PORT' --plan free` → live in 213 s, and `GET /` → `{"message":"Hello World"}`.
