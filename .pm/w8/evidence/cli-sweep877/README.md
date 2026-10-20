# CLI sweep 877 — docker runtime `envSpecificDetails` round-trip (live, 2026-10-20)

Disposable docker-runtime free web service created, patched, deleted (204, then GET 404). No new bug filed.

- Create with `envSpecificDetails {dockerfilePath: "Dockerfile", dockerContext: "."}` → 201; GET emits the docker block (`dockerCommand: ""`, context, path — the empty command present-but-empty in the docker spelling, per `render.go`'s docker branch).
- `PATCH envSpecificDetails.dockerfilePath` → 200, read back `docker/Dockerfile` with context preserved.
