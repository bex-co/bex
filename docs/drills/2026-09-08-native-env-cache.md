# Native build-env cache invalidation — 2026-09-08

Proves the w7/m87 contract: with registry caching enabled, an unchanged source commit whose effective native build environment changes from MESSAGE=A to MESSAGE=B must produce an artifact containing B; a subsequent unchanged rebuild may reuse the fresh cache. Secret mount contents alone must not keep baking A.

## Failure mode (reproduced)

Docker documents that BuildKit secret mount contents are not part of an instruction's cache key ([build secrets](https://docs.docker.com/build/cache/invalidation/#build-secrets)). bex's native Dockerfile historically passed the whole effective environment only through `--mount=type=secret,id=render-env`, so a warm rebuild at a fixed commit could reuse the env-dependent `RUN` after values changed.

Local BuildKit reproduction (orbstack / Docker 29.4.0, `busybox:1.37.0`, same Dockerfile text shape as `nativeDockerfile`):

| Step | Revision token in RUN | Secret MESSAGE | Artifact `message.txt` | Notes |
| --- | --- | --- | --- | --- |
| Cold | `1` | A | A | fills cache |
| Warm, env-only change | `1` (unchanged) | B | **A (stale)** | 3× `CACHED` — the bug |
| Warm, bumped revision | `2` | B | **B** | RUN re-executes |

Synthetic values only; no tenant credentials. Host-local images were removed after the run (`m87repro:a` / `:stale` / `:b`).

## Fix

The operator persists an opaque monotonic revision on the App-owned `<app>-native-env` Secret (`app.bex.co/native-env-revision`) and embeds `: bex-native-env-rev=<rev>` inside the native env-dependent `RUN`. A keyed HMAC equality token (`app.bex.co/native-env-input`, App UID as key) covers merged Secret bytes plus build-relevant literals so reconciles and restarts keep the same revision when the effective environment is unchanged. The token never enters BuildKit, generated Dockerfiles, or image metadata; raw values never appear in the Dockerfile.

Unit coverage (default `make test` path):

- `TestNativeDockerfileEnvRevisionBustsCacheKey` — values stay out of the Dockerfile; distinct revisions change the RUN cache key.
- `TestProjectNativeBuildEnvRevisionStabilityAndInvalidation` — Secret value, literal overlay, and unlink bump the revision; unchanged input does not; annotations carry no raw values.
- `TestProjectNativeBuildEnvLiteralsAlonePersistRevision` — literals-only still persist a Secret so the revision survives restarts.

## Cluster drill

`TestRegistryBuildCacheDrill` gains `BEX_CACHE_DRILL_WORKLOAD=native-env`: three generations at a fixed commit (MESSAGE A → B → B), asserts the built image's `/opt/render/project/src/message.txt` via an in-cluster reader Pod, and cleans up Jobs/Pods under UID preconditions. Run from `lego/operator` with the same opt-in env as the m86 drill plus the native-env workload flag.

The local kind/CAPD kubeconfig available during this session failed TLS verification (`x509: certificate signed by unknown authority`), so the in-cluster drill was not re-executed here. The local BuildKit table above is the dated artifact-level proof for A→B and the revision fix; re-run the opt-in drill once the mock cluster is healthy to capture Job timings and BuildKit `CACHED` counts on the real build plane.

## Scope boundary

Production `BEX_BUILD_CACHE` enablement remains the separate inbox decision `w7/043`. Clear-cache deploys are `w7/m88`.
