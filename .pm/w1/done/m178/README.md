# w1 · m178 — Dockerfile builds receive the service's env vars as build args and its secret files as build secrets

**Worker:** worker1 **Goal:** a Dockerfile service's build sees its effective env vars as build args and its secret files as BuildKit secrets at Render's ids, so a Render Dockerfile builds the same on bex. **Status:** done (2026-10-17)

## Tasks (in order)

| id   | title                                                                                        | est | depends_on |
| ---- | -------------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Record Render's Docker build contract for env vars and secret files — **DONE** | 30m | —          |
| t002 | Pass the effective env to Dockerfile builds as build args, with no value in the Job spec — **DONE** | 1h  | t001       |
| t003 | Expose secret files to Dockerfile builds as BuildKit secrets under Render's id rule — **DONE** | 45m | t001       |
| t004 | Count env and secret-file revisions in Dockerfile release identity; keep secrets out of the cache — **DONE** | 30m | t002, t003 |
| t005 | Document the Dockerfile build contract in ADR004 and ADR018 — **DONE** | 20m | t004       |
| t006 | Live check on production after the operator deploys — **DONE** | 40m | t004       |
| t007 | Render parity — **DONE** | 20m | t005, t006 |
| t008 | Simplify — **DONE** | 15m | t007       |
| t009 | Test coverage — **DONE** | 40m | t007       |
| t010 | Closeout — **DONE** | 10m | t009       |

## Definition of done

- A Dockerfile service's build resolves `ARG <NAME>` to the value saved through the env-vars API (own and linked-group sources, precedence per t001) and reads each secret file at `/run/secrets/<render-id>` via `RUN --mount=type=secret,id=<render-id>`.
- No env value or secret-file content appears in the build Job/Pod spec, the exported build cache, or the image's secret-mount layers.
- Native and buildpack builds are unchanged (their Job specs byte-identical in tests).
- The live check (t006, amended 2026-10-17 from dev-1 to production after deploy: dev-1 shares the cluster's single operator) passes and the fixture is deleted. The generated BuildKit container already passed locally against `examples/docker-build-env`.

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

## Hosted probe — qa pass f26a22 (2026-10-19, pin `db7e50ced338`)

This extends t006's basic check with the decisions recorded in `docs/render-artifacts/docker-build-env.md`. The fixture was `qa-20261019-f26a22-dbe` (`srv-dbasval212nc73a1i79g`, Free Docker, `https://github.com/bex-co/bex` root `examples/docker-build-env`, `autoDeploy: no`) in workspace `bex-canary`. It was deleted afterwards, along with its group, and only that run's session was revoked. All writes used GraphQL `patchServiceEnvironment`.

| Step | Result |
| --- | --- |
| Create with `GREETING=qa-hello-f26a22` and `.env` (5 bytes) | `/greeting` → `qa-hello-f26a22`, `/env-bytes` → `5` |
| Add secret files `a.b` and `a_b`, `saveMode:"rebuild"` | Saved (`rolledOut:true`). The deploy closed `build_failed` within 3 s with `failureReason` "dispatching build: build: secret files "a.b" and "a_b" both become Docker build secret id "a_b"; rename one of them", shown the same on the deploy page header and log. The live release kept serving. |
| `GREETING=qa-hello-second`, delete `a_b`, `saveMode:"deploy"` | Live. `/greeting` still `qa-hello-f26a22`: the existing build is redeployed, as on Render. |
| Empty patch, `saveMode:"rebuild"` | `/greeting` → `qa-hello-second` |
| Link env group `qa-20261019-f26a22-grp` (`GREETING=from-group`, `.env` 17 bytes), rebuild | `/greeting` → `qa-hello-second`, `/env-bytes` → `5`: the service's own values win. Linking did not deploy (`autoDeploy: no`). |
| Delete the service's `GREETING` and `.env`, rebuild | `/greeting` → `from-group`, `/env-bytes` → `17`: group variables and files reach the build (the inferred scope rule). |

Every recorded decision holds live. One adjacent finding, outside this milestone's scope, was filed as w4/m235: the URL answers Traefik's raw `404 page not found` during the first build.
