# CLI sweep 1287 — CLI smoke on pin `a3f66521f93d`, and a live runtime switch (w5/m152) (2026-10-21)

Production `https://api.bex.co/v1/`, bex-canary, HEAD-built pinned CLI.

- **Read-only CLI smoke.** `whoami`, `workspaces`, `workspace current`, `services`, `pg list`, `kv list`, `projects`, `deploys list`, `services instances`, `logs`, `jobs list`, `workflows list` and `ea sandboxes list`, all with `-o json`, exit 0 with parseable JSON in 0.3–1.0 s. No regression.
- **Runtime switch, live.** w5/m152's closeout notes that "node to docker rebuilds and serves" was proven only at spec level, so it was exercised live:
  - `bex services create --runtime python --root-directory flask/app --repo https://github.com/docker/awesome-compose --branch master --build-command "pip install -r requirements.txt" --start-command "python app.py" --env-var PORT=8000 --plan free` → `live`, `serviceDetails.port: 8000`. That `PORT` selects the port confirms w5/m149 live.
  - `bex services update <id> --runtime docker` is refused locally by the pinned CLI ("cannot switch runtimes via the CLI; use the API directly…"). That is upstream behavior, not bex's.
  - REST `PATCH serviceDetails.runtime: docker` → 200, and a new deploy rebuilt from `flask/app/Dockerfile` (BuildKit `FROM docker.io/gloursdocker/docker…` stage), `live` in 77 s. `GET /` returned 200 `Hello World!`.
  - **Observation (not filed; Render behavior unverified):** after the switch the service reads back `envSpecificDetails.dockerCommand: "python app.py"`, so the native start command became the Docker Command and overrides the Dockerfile's `CMD`. It is harmless here. For a Dockerfile whose image lacks the native command (for example a Go binary image after a Node service's `npm start`), the switched service would not start. Render keeps `startCommand` (native) and `dockerCommand` (docker) as separate fields; whether its runtime switch carries one into the other is not documented.
  - `srv-dbcbl29akels738t2lg0` was deleted (204 → 404).
- New production pin `4b6687b5bc18` (bd03f852b, 13:17 UTC) landed during this sweep.
