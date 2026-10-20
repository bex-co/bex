# CLI sweep 1228 — live tail filters, services TUI, and the blank deploy SHA (live, 2026-10-20)

One disposable free web service (`qa-20261020-451090-tail`, `srv-dbbfv719a20c7385r9m0`, image `docker.io/ealen/echo-server:0.9.2`) created with `bex services create … --env-var QA_MARK=…`, live in 43 s, deleted with `bex services delete --confirm` (GET 404 verified). HEAD checkout build at `94533d795` (`vdev`), isolated config, workspace `bex-canary`, human device-login grant. Deployed pin `94533d795067`.

- **Env reaches the process:** `GET /?echo_env_body=QA_MARK` → 200 with the marker.
- **`bex logs --tail` request streaming (w4/m237, now deployed):** seven concurrent `--tail -o json` streams, nine marker requests (3×GET 200, 3×POST 200, 3×GET 404). Every filter streamed exactly its subset live: unfiltered 18 marker lines (9 request + 9 app), `--type request` 9, `--path /<mk>-a` 3, `--status-code 404` 3, `--method POST` 3, `--type app --text <mk>` 9, `--host <slug>` 9. No errors; streams ended on SIGTERM.
- **Services TUI (PTY 160×45):** list, `/` search, palette and deploy list render; the palette's “Open the Render Dashboard” copy is the documented upstream runtime-copy residual class, and the OAuth-origin Dashboard links now 302 to `dashboard.bex.co` (w8/089 edge redirect live).
- **Finding — deploy `image.sha` is never reported.** The TUI deploy row reads `Image: docker.io/ealen/echo-server:0.9.2 SHA:` (blank). REST `GET /services/{id}/deploys` returns `"image": {"ref": "docker.io/ealen/echo-server:0.9.2"}` with no `sha`. Filed as [w8/m122](../../m122/README.md).
- Not findings: no `RENDER_*` default variables reach the process (ADR018's deliberate exclusion); the first deploy's `trigger: other` / `bexTrigger: create` is the documented mapping.
