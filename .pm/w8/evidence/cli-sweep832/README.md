# CLI sweep 832 — blueprint generate + w8/133 regression spot-check (live, 2026-10-20)

Disposable free web service created, generated from, deleted (204, then GET 404). No new bug filed.

- `POST /blueprints/generate {"serviceIds":[…]}` → 200 with a `render.yaml` manifest describing the fixture (`image.url`, `plan: free`, `runtime: image`) and `filename: "render.yaml"`.
- Empty selection → the w8/133 fix's named 400 `select at least one resource to generate a Blueprint from` (still live at pin `ca30db3a3646`).
- Synthetic service id → coded 404.
