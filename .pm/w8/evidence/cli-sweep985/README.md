# CLI sweep 985 — dockerCommand / preDeployCommand field placement (live, 2026-10-20)

Two disposable free web services created and deleted (204s). No new bug filed.

- `serviceDetails.dockerCommand` → coded 400 naming the unknown field; the correct Render placement `serviceDetails.envSpecificDetails.dockerCommand` → 200 and reads back verbatim. The refusal teaches the shape.
- `healthCheckPath` PATCH → 200, reads back.
- `preDeployCommand` on an image-runtime service is accepted and stored (visible both top-level in `serviceDetails` and inside `envSpecificDetails`); deploy-time semantics were not probed, so no claim is made beyond storage.
