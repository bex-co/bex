# docker-build-env

A Docker web service that only builds when the service's environment reaches the Docker build the way Render passes it: env vars as build args, secret files as BuildKit build secrets ([w1/m178](../../docs/render-artifacts/docker-build-env.md)).

Deploy it as a Docker service with root directory `examples/docker-build-env`, then set:

- env var `GREETING` (any non-secret test value: a consumed `ARG` is recorded in the image history, on Render too);
- secret file `.env` (any content).

Then:

- `GET /greeting` returns the `GREETING` value baked in at build time;
- `GET /env-bytes` returns the byte size of `.env` as the build saw it.

Without `GREETING` the build fails at `test -n`; without `.env` it fails at the `required=true` secret mount.
