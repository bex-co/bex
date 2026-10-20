# CLI sweep 851 — image-service command override round-trips as `dockerCommand` (live, 2026-10-20)

Disposable free image-backed web services created, patched, deleted (204s; GETs 404). No new bug filed.

- `PATCH {"serviceDetails":{"envSpecificDetails":{"startCommand":"echo qa851"}}}` on an image-backed web service → 200, and both the PATCH response and a full GET read it back as `envSpecificDetails.dockerCommand: "echo qa851"` — for a prebuilt image, `startCommand` and `dockerCommand` are the same CMD-override field (w4/188), and the read shape uses the docker spelling (`lego/backend/internal/apps/render.go` `envSpecificDetails`). Patching `dockerCommand` directly updates the same field.
- A first pass looked like a silent accept-and-drop; that was the harness's 1500-char output truncation hiding `serviceDetails` — a full-parse re-run (after fixing a token-regex bug in the new helper, the same defect behind sweep 831's spurious 401s) shows the stored value. Recorded as harness error, not filed.
