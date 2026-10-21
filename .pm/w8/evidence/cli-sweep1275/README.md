# CLI sweep 1275 — Render's official SvelteKit, Rails 7 and Next.js examples (live, 2026-10-21)

Production `https://api.bex.co/v1/`, pin `9cace6b6e25d`, bex-canary, human device-login grant, HEAD-built pinned CLI.

- `bex blueprints validate` on `render-examples/{rails-7,sveltekit,nextjs-hello-world}/render.yaml` → all `valid: true`. The plan summary lists only resource names, which is exactly Render's `ValidationPlanSummary` shape.
- **Fixture mistake, recorded for the cleanup ledger.** `render-examples/sveltekit` declares no `plan`, so the Blueprint created `srv-dbc79apv5vjs73b9hco0` on Render's default **`starter`** (paid). Bex matches Render here, since a service without `plan` defaults to starter. The service and Blueprint `blp-dbc79apv5vjs73b9hcmg` were deleted about 2 minutes after creation (204 → 404). From now on, every Blueprint service and database is checked for `plan: free` before connecting.
- The same app recreated on the free plan with the CLI: `bex services create --type web_service --plan free --repo https://github.com/render-examples/sveltekit --runtime node --build-command "npm install && npm run build" --start-command "node build/index.js"` → `live` in 217 s. `/`, `/about` and `/sverdle` return 200, an unknown path returns the app's 404. `srv-dbc79spv5vjs73b9hcr0` was deleted (204 → 404).
- `render-examples/rails-7` was not deployed. Its `bin/render-build.sh` ends with `bundle exec rake db:migrate`, which is the w8/158 pattern, and it declares no plan, which would default to a paid plan.

No new bug.
