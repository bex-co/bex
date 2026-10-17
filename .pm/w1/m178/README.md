# w1 · m178 — Dockerfile builds receive the service's env vars as build args and its secret files as build secrets

**Worker:** worker1 **Goal:** a Dockerfile service's build sees its effective env vars as build args and its secret files as BuildKit secrets at Render's ids, so a Render Dockerfile builds the same on bex. **Status:** todo

## Tasks (in order)

| id   | title                                                                                        | est | depends_on |
| ---- | -------------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Record Render's Docker build contract for env vars and secret files                           | 30m | —          |
| t002 | Pass the effective env to Dockerfile builds as build args, with no value in the Job spec      | 1h  | t001       |
| t003 | Expose secret files to Dockerfile builds as BuildKit secrets under Render's id rule           | 45m | t001       |
| t004 | Count env and secret-file revisions in Dockerfile release identity; keep secrets out of the cache | 30m | t002, t003 |
| t005 | Document the Dockerfile build contract in ADR004 and ADR018                                   | 20m | t004       |
| t006 | Live check on dev-1                                                                           | 40m | t004       |
| t007 | Render parity                                                                                 | 20m | t005, t006 |
| t008 | Simplify                                                                                      | 15m | t007       |
| t009 | Test coverage                                                                                 | 40m | t007       |
| t010 | Closeout                                                                                      | 10m | t009       |

## Definition of done

- A Dockerfile service's build resolves `ARG <NAME>` to the value saved through the env-vars API (own and linked-group sources, precedence per t001) and reads each secret file at `/run/secrets/<render-id>` via `RUN --mount=type=secret,id=<render-id>`.
- No env value or secret-file content appears in the build Job/Pod spec, the exported build cache, or the image's secret-mount layers.
- Native and buildpack builds are unchanged (their Job specs byte-identical in tests).
- The dev-1 live check (t006) passes and the fixture is deleted.

## Evidence (verified 2026-10-17 at `2c95fd35b`)

- `lego/operator/internal/build/build.go:947-960`: `--secret id=render-env` and the secret-file mounts are added only when `o.Builder == BuilderNative`; the Dockerfile branch passes neither. Volumes (`:1024-1030`) are native-only too. `grep -rn build-arg lego --include=*.go` finds nothing.
- Render: render.com/docs/docker-secrets, fetched 2026-10-17 — "Render injects your service's environment variables as build args with the same keys and values"; secret files are build secrets, id = filename with non-`[A-Za-z0-9_-]` → `_`, default `/run/secrets/<id>`.
- No recorded divergence: ADR004, ADR018 (secret-files row :128 is runtime-only) and ADR089 say nothing about Dockerfile build env.

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w1` 2026-10-17 item 1, verified the same day and approved by the user for filing. Originally noted in `w1/done/121.md:16` ("a separate parity gap, not filed here").
- **Goal linkage:** the Render-alternative core (ADR008) and the ADR089 migration path for existing Render Docker services.
- **Expected outcome:** a Render Dockerfile that relies on `ARG` (e.g. `NEXT_PUBLIC_*`) or secret mounts builds with the right values instead of silently building with empty ones.
- **Why now:** it is a silent wrong build for exactly the users being migrated, and the native half of the same feature (w4/m163's file projection, w1/121's env sources) just shipped, so the code to reuse is fresh.
- **Render parity included:** build behavior is tenant-visible and the ADR018 rows change; no new API field is expected.
- **Related, not taken over:** `w4/blocked/m163` (native build secret files; its live replay stays there). Private CNB base images remain in FUTURE-MAYBE.
