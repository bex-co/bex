# w4 · m92 — Preserve exact environment values in native builds

**Worker:** worker4 **Goal:** Native build commands receive the same environment bytes as the API and running service. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Preserve padded Base64 and trailing newlines in the native decoder | 40m | — |
| t002 | Fix literal preparation and verify the shared runtime consumers | 35m | t001 |
| t003 | Render parity and adapter contract check | 20m | t002 |
| t004 | Simplify | 15m | t003 |
| t005 | Test coverage with executed preparation and decoding | 30m | t003 |
| t006 | Closeout with live byte comparison | 15m | t004, t005 |

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
