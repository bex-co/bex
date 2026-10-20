# CLI sweep 759 — blueprint generate, success path (live, 2026-10-19)

Read-only generation against the baseline `hello-go`; nothing created, manifest body not recorded beyond its shape.

- `POST /blueprints/generate {"serviceIds":["<baseline>"]}` → 200 `{"filename", "manifest"}`; the manifest (173 chars) names `hello-go` with its repo/branch/plan — the w8/133 endpoint's happy path works on the deployed fix pin.

No bug filed.
