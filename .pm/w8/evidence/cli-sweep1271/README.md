# CLI sweeps 1271–1272 — real-world Blueprints from the sweep 1270 corpus deployed live (2026-10-21)

Production `https://api.bex.co/v1/`, pin `9cace6b6e25d`, bex-canary, human device-login grant. Two corpus files that bex validates were connected as Blueprints (`POST /v1/blueprints`), and each first sync was followed to a terminal deploy state. No new bug.

| Repo | Blueprint shape | Result |
| --- | --- | --- |
| `whoshotu/wassup-chat` | static, `npm ci && npm run build`, `dist`, SPA rewrite `/*` → `/index.html` | `live` in 243 s. `GET /` and `GET /some/deep/route` both return 200 with the same 2491-byte `index.html`, so the rewrite works |
| `mcansh/remix-fastify` | free Node web, `rootDir: ./examples/basic`, corepack + pnpm workspace | `build_failed` in 170 s: pnpm cannot resolve `@mcansh/react-router-fastify@workspace:` because the workspace root sits outside `rootDir` |

The remix-fastify failure matches Render. Render's monorepo guide says "Files outside your service's root directory are not available to the service at build time or at runtime", so this Blueprint cannot build on Render either. Bex's native build copies only `rootDir` (`COPY . .` with that context), which is the same contract. Not filed.

Both Blueprints were created with `autoSync: true` even though the body sent `false`. `POST /v1/blueprints` is a bex-native route whose body has no `autoSync`; the field is update-only on every surface. Each was PATCHed to `false` within a minute, and this was already recorded as a non-finding in sweep 1265.

Cleanup: both Blueprints DELETE 204 → GET 404; services `srv-dbc5dphv5vjs73b9hbng` and `srv-dbc5fo9v5vjs73b9hbs0` DELETE 204 → GET 404.

## Sweep 1272

| Repo | Blueprint shape | Result |
| --- | --- | --- |
| `crazybber/bingai` (branch `master`) | free Go web, `go build -ldflags="-s -w" -tags netgo -trimpath -o app`, `./app`, `buildFilter` paths and ignoredPaths | `live` in 284 s; `GET /` 200. The app is a Bing proxy, so it was deleted right after the check: Blueprint 204 → 404, `srv-dbc5ikm5u6js73eso400` 204 → 404 |
| `nzolivier/app` | free Python web | `POST /v1/blueprints` → 400 `file_not_found`: the repo's file is named `render.yaml ` (trailing space), so this is correct |

`rekha0suthar/pdf-question-answer-ai` was skipped because its backend installs `torch`/`transformers`, which is too heavy for a free QA fixture. No new bug.
