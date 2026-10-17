# Render: what a Docker build receives (env vars and secret files)

Captured 2026-10-17 for `w1/m178` from Render's public documentation. No live Render build was run.

## Sources

| Source | Fetched | What it settles |
| --- | --- | --- |
| [render.com/docs/docker-secrets](https://render.com/docs/docker-secrets) | 2026-10-17 | Build args from env vars; secret files as BuildKit build secrets, their id rule and default path |
| [render.com/docs/configure-environment-variables](https://render.com/docs/configure-environment-variables) | 2026-10-17 | Service-vs-group precedence; the three save choices and which one rebuilds |

## Environment variables become build args

- "Render injects your service's environment variables as build args with the same keys and values." A Dockerfile reads one by declaring `ARG <NAME>`; Docker records an `ARG` the Dockerfile consumes in the image history, which is why Render adds "We recommend against using `ARG` instructions for secrets." Undeclared build args are not consumed and not recorded.
- **Where a consumed value shows up (bex observation, 2026-10-17):** running bex's generated BuildKit container (`moby/buildkit:v0.30.0`) on the `examples/docker-build-env` fixture recorded `ARG GREETING=<value>` and `RUN |1 GREETING=<value> …` in the image history, and BuildKit's progress log printed the RUN line with the value expanded. A file secret left no path in any layer. This is BuildKit's behavior, and therefore Render's too. It is why Render's warning applies, and nothing in bex adds a value the Dockerfile didn't consume.
- **Scope:** the service's effective environment, meaning its own variables plus those of its linked environment groups. The Docker page doesn't separately name groups. The env-vars page treats group variables as part of the service's environment, so bex includes them. _Inferred default._
- **Precedence:** "If a service defines an environment variable in its individual settings, that value always takes precedence over any linked environment groups that also define the variable." Between two groups Render "does not guarantee its precedence behavior". bex keeps the runtime `envFrom` order it already uses for native builds and the running pod, so build-time and runtime values never disagree.

## Secret files become build secrets

- Each secret file "is available to the build as a build secret" and is read through `RUN --mount=type=secret,id=<SECRET_ID>`. "Unlike build args, secret mounts aren't persisted in your built image."
- **Id rule:** the filename with every character that is not a letter, digit, `_` or `-` replaced by `_` (Render's examples: `.env` → `_env`, `credentials.json` → `credentials_json`).
- **Path:** "If you omit `dst`, the file is at `/run/secrets/SECRET_ID`"; `dst` may name any path. The file "is available only while its instruction runs", root-only unless the mount sets `uid=`. "With `required=true`, your build fails if no secret file has the ID."
- **Collisions:** two filenames that map to the same id (for example `a.b` and `a_b`) are not addressed by the docs. bex fails the build with a message that names both files, rather than silently dropping one. _bex decision._

## When new values reach the build

The env-vars page lists three save choices:

- **Save, rebuild, and deploy:** "Render triggers a new build for your service and deploys it with the new environment variables." The new values reach the build args.
- **Save and deploy:** "Render redeploys your service's existing build with the new environment variables." The image keeps the build args it was built with.
- **Save only:** no deploy until the next one.

bex's environment editor offers the same three choices (`dashboard/src/features/services/locales/en.ts`, `services.environmentSave*`). A rebuild is a fresh artifact: deploy verbs bump `spec.restartedAt`, which is part of every builder's artifact identity (`lego/operator/internal/controller/release_identity.go`). So a Dockerfile build sees the values saved when it was started, exactly as on Render. No new identity input is needed.
