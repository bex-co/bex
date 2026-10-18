# Docker runtime + deploy-override refusals — sweep464

Passing functional coverage, no new issue. Same isolated session as sweeps 454–463, `bex-canary` (+ personal for Postgres), HEAD checkout build (`6fc5c3577`), 2026-10-18T05:46–05:56Z.

Docker runtime from Git (`bex-co/bex` `examples/hello-go` via `--root-directory`, `runtime/env` read back `docker`): build → `live`, HTTP 200 `OK`. Deploy-override refusal is exact and side-effect-free: `--image docker.io/library/nginx:alpine` on a `gcr.io/google-samples/hello-app` service → named 400 ("only the tag or digest can change per deploy") and no deploy row created (re-verifies w8/m49 + w8/042 live). `--runtime` switch on update refuses client-side with the upstream guard text. Datastore free-plan refusals are coded and state-preserving: `--disk-size-gb 5` → `POSTGRES_STORAGE_PLAN_UNSUPPORTED` (disk stays 1 GB), `--disk-autoscaling` → `POSTGRES_DISK_AUTOSCALING_PLAN_UNSUPPORTED`, `--connection-pool pgbouncer` → `POSTGRES_CONNECTION_POOL_PLAN_UNSUPPORTED`; non-multiple-of-5 sizes refuse client-side; a bogus `--memory-policy` refuses client-side with the full enum.

Cleanup: exact-owned `srv-dba5u5r6gvbc73encbv0`, `srv-dba5u636gvbc73encc10`, `dpg-dba60236gvbc73encca0` deleted by ID (404 / named not-found verified); baseline byte-identical. No plan mutations. Session remains active; the loop continues from sweep465.
