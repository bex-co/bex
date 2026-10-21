# CLI sweep 1279 — modern Dockerfile features and Render's static-header globs (live, 2026-10-21)

Production `https://api.bex.co/v1/`, pin `9cace6b6e25d`, bex-canary, human device-login grant.

- **BuildKit frontend features.** `docker/awesome-compose` `flask/app/Dockerfile` (branch `master`) uses `# syntax=docker/dockerfile:1.4`, `FROM --platform=$BUILDPLATFORM`, `RUN --mount=type=cache`, `RUN <<EOF` heredocs, and `COPY --from=gloursdocker/docker / /` from a third-party image, with the final `dev-envs` stage as the default target. It was created as a free Docker web service through REST (`dockerContext: flask/app`, `dockerfilePath: flask/app/Dockerfile`, no `rootDir`, bex `port: 8000`) → `live` in 128 s, and `GET /` returned 200 `Hello World!`. `srv-dbc8rnm5u6js73eso6ig` was deleted (204 → 404).
- **Static header path globs.** Render's [static-site headers](https://render.com/docs/static-site-headers) define `/*` (all paths), `/blog/*`, `/**/*` (paths with at least two slashes), `/*.css` (top level only) and `/**/*.css` (nested only, not `/tokens.css`). Bex's `matchHeaderPattern` (`lego/operator/internal/staticserver/staticserver.go:636`) encodes exactly these distinctions, and last-match-wins via `Set` is its documented precedence. Render documents no precedence rule, so nothing to compare.
- **Render CLI reference sweep.** Each command in [render.com/docs/cli-reference](https://render.com/docs/cli-reference) maps to a graded row in `docs/cli-compatibility-checklist.md`. Workflows and jobs create are recorded non-goals.

No new bug.
