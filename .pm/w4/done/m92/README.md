# w4 · m92 — Preserve exact environment values in native builds

**Worker:** worker4 **Goal:** Native build commands receive the same environment bytes as the API and running service. **Status:** done

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Preserve padded Base64 and trailing newlines in the native decoder | 40m | — — **DONE** |
| t002 | Fix literal preparation and verify the shared runtime consumers | 35m | t001 — **DONE** |
| t003 | Render parity and adapter contract check | 20m | t002 — **DONE** |
| t004 | Simplify | 15m | t003 — **DONE** |
| t005 | Test coverage with executed preparation and decoding | 30m | t003 — **DONE** |
| t006 | Closeout with live byte comparison | 15m | t004, t005 — **DONE** |

## Outcome (2026-09-06)

Shipped. The generated native-build decoder now reads each `KEY=BASE64` record whole (`IFS= read -r`, split at the first `=`), so Base64 padding survives, and decodes through a sentinel-guarded command substitution so trailing newlines survive; invalid Base64 aborts the build before the tenant command runs, with no partial export and no value echoed. The literal producer strips exactly printenv's one added terminator (`${value%?.}` after the sentinel guard), never the user's own trailing newlines; the Secret-file path was already byte-exact and is unchanged.

- **Executed verification against the real pinned images:** a Docker harness ran the generated preparer inside `busybox:1.37.0@sha256:9db7b599…` (the exact digest-pinned preparer image) and the generated RUN shell inside bookworm bash/coreutils (the base of all six toolchain images) — 17/17 pass across empty, `a`/`ab`/`abc` (padding 2/1/0, no `base64: invalid input`), nine-byte `qa-build\n` (9 bytes at build), `two\n\n`, embedded newline, and shell-metacharacter values, each via both the literal and Secret-backed path, plus invalid-Base64 failing (rc=1) with no build output. The old decoder was re-executed as a control and reproduces the loss (9-byte value → 8 bytes).
- **t002:** one `nativeDockerfile` caller, one `nativeBuildPreparer` caller confirmed; the codec fix is global to all six native toolchains and every App kind that uses the native strategy (web/private/worker/cron + static's extract-only image). Duplicate-key literal-over-Secret precedence, PORT/`BEX_NATIVE_*` filtering, and non-login-shell PATH behavior preserved (existing tests still green).
- **t003:** REST/GraphQL/MCP writers, the secret store (`resolveValue` verbatim; only **keys** are trimmed), and runtime projection (`upsertSecret` bytes → EnvFrom) confirmed normalization-free — no adapter change needed, no store change made to mask the loader bug.
- **t005:** `TestNativeEnvironmentRoundTrip` executes the generated preparer + RUN shells over the full fixture matrix and fails against the original loader; `TestNativeEnvironmentRejectsInvalidBase64` proves fail-closed decoding. `make test` (operator + envtest) and `make lint` (all four modules) green.
- **Limit (t006):** the live prod re-probe of the DoD journey (fresh `qa-*` service on dashboard.bex.co against the **deployed** fix) needs this ship to roll out first — deferred to the next live QA pass; the byte-level DoD checks were executed mechanically against the real pinned images instead. The original live fixtures from the filing pass were already deleted at filing time.

## Definition of done

- Repeat t001's real free Go service journey: set MESSAGE to the nine bytes `qa-build\n`; the API returns that exact string, both build and runtime logs report 9 bytes, and the external URL returns hex `71 61 2d 62 75 69 6c 64 0a`.
- Repeat the eight-byte `qa-build` control: build and runtime both report 8, with no decoder `base64: invalid input` warning.
- Reload the deploy detail, search logs for `QA_`, and verify the same counts; a new manual deploy preserves them.
- Execute the generated preparer/decoder against literal and Secret-backed fixture values, covering empty values, one/two trailing newlines, embedded newlines, and Base64 with zero/one/two padding characters. Preserve literal-over-Secret precedence, reserved-key filtering, and absence of secret values from generated Dockerfiles/image metadata.
- Remove the prefixed live fixtures after verification. Other runtimes and service kinds have the explicit verification work in t002; they were not all exercised live by this hunt.

## Source + Goal linkage

- **Source:** user's continuous `$qa-find-bugs w4` request, 2026-09-06, first loop pass. Live major finding: API/runtime contain 9 bytes, native build receives 8. Exact probes and identifiers are in t001; screenshot `.playwright-mcp/qa-build-env-3.png` exists locally and is gitignored.
- **Goal linkage:** faithful service configuration and Render-compatible builds, ADR008 / ADR004 in-cluster build contract / ADR060 build reliability.
- **Expected outcome:** build credentials/configuration are not silently normalized differently from runtime configuration.
- **Why now:** production reproduces the loss with a valid non-secret fixture; the decoder is shared by six native toolchains. A decoder-only change would expose the literal producer's extra newline, so the producer and consumer must change together.
- **Render parity included:** user-facing environment/build semantics span existing REST/GraphQL/MCP writers and dashboard logs. [Render documents rebuilding with the saved environment](https://render.com/docs/configure-environment-variables); exact upstream newline behavior was not probed.
- **Dedupe:** searched open/done board for trailing newline, command substitution, Base64 padding/invalid input, and native-build env; reviewed open milestones across workstreams and DO_NOT_DO. w6/done/m45 fixed PATH and create-time env-store visibility, not this codec. Native source history through main `1830b65c3` still contains the loss; no deploy-lag fix. w7/done/m86 concerns cache performance; this fresh build explicitly executed the RUN step.
