# w9 · m93 — Read/write round-trip parity: prove every service shape a client can create, it can also update

**Worker:** worker9 **Goal:** Turn the one-field w4/052 fix into a class-level guard so no future service-read omission can silently strand the official CLI's partial `services update` again. **Status:** in progress — t001/t002/t004/t005/t006 implemented + green (`internal/apps` matrix + totality + cross-surface guards, mutation-checked non-vacuous); t003 verifier legs written (`image-healthcheck`, `web-builder-roundtrip`) but **live run pending** (needs the CLI + a cluster); the prebuilt-image round-trip residual flagged in w4/052 was resolved here (read derives runtime `image`). t007 closeout is gated on `/ship` + the live verifier run.

## Tasks (in order)

| id   | title                                                                    | est | depends_on               |
| ---- | ------------------------------------------------------------------------ | --- | ------------------------ |
| t001 | Read-side round-trip completeness guard (create-matrix readback)         | 45m | —                        |
| t002 | `effectiveRuntime` totality guard driven off the build-strategy matrix   | 30m | —                        |
| t003 | Fixture diversity in the live services parity verifier                   | 45m | —                        |
| t004 | Render parity check                                                       | 20m | t001, t002, t003         |
| t005 | Simplify                                                                  | 20m | t004                     |
| t006 | Test coverage — prove every new guard is non-vacuous                     | 30m | t004                     |
| t007 | Closeout                                                                  | 10m | t006                     |

## Definition of done

- A backend test asserts, for **every** service build-strategy shape the create/normalization path can persist (docker-via-builder, docker-explicit, image-no-runtime, image-explicit, each native runtime, buildpack, static site), that `GET`/read output carries a build contract sufficient to reconstruct a **no-op** `services update`: `serviceDetails.runtime`/`env` is present and non-empty for every runnable (non-static) type, equals the runtime bex would recompute for the current spec (so an explicit `--runtime` resend is never a "switch"), and the runtime-keyed `envSpecificDetails` shape agrees with it. Any residual gap the matrix surfaces (e.g. prebuilt-image round-trip) is either fixed or recorded as an accepted divergence with captured Render evidence — never left silent.
- A focused guard proves `effectiveRuntime` is **total**: no `(type, source, builder)` combination the create API accepts returns an empty runtime for a runnable type except the explicitly enumerated no-runtime shapes (buildpack, static). A newly added builder/type that forgets its runtime mapping trips this test.
- `scripts/cli-services-parity-verify.sh` creates and exercises a Dockerfile-via-builder web service (no `--runtime`) **and** an image-no-runtime web service, running the same `services update --health-check-path` (+ a second `serviceDetails` field) leg against each — the monoculture `--runtime go` fixture that hid w4/052 can no longer hide its successor.
- The three suites (`operator make test`, `backend go test ./...`, `dashboard yarn test`) and `make lint` stay green; each new guard is shown to fail when the behavior it protects regresses (t006).

## Source + Goal linkage

- **Source:** the just-shipped **w4/052** fix (`cca2f571d`, `internal/apps.effectiveRuntime` — a Dockerfile web service left `spec.runtime` empty, so `GET /v1/services` omitted `serviceDetails.runtime`/`env` and the upstream Render CLI refused a partial `services update` client-side). Post-fix root-cause discussion with the user: the bug was not a typing/OpenAPI-shape defect (the field is schema-valid and optional) but a **read/write round-trip** gap — two internal encodings of one external fact (`spec.builder` vs `spec.runtime`) plus a parity verifier whose fixture only ever set the runtime explicitly, so it never exercised the failing path.
- **Goal linkage:** Render API/CLI parity ([docs/ADR018-render-parity.md](../../../docs/ADR018-render-parity.md), [docs/cli-compatibility-checklist.md](../../../docs/cli-compatibility-checklist.md)) — the fifth surface is the unmodified `render-oss/cli` driven against bex-api ([`.pm/DO_NOT_DO.md`](../../DO_NOT_DO.md): CLI gaps become bex-api work, tracked in the checklist owned by w9). A read that cannot round-trip through the real client is a parity defect the checklist's green marks must not be able to hide.
- **Expected outcome:** the specific class of bug — "read output is insufficient to reconstruct a valid write" — is caught in CI (backend guard) and in the live verifier (real CLI), so `[x]`-marked `services update` flags reflect coverage of every build strategy, not just the one that happens to set `runtime`.
- **Why now:** the mechanism (`effectiveRuntime`) and the exact failure are freshly understood from w4/052; encoding the invariant while the context is warm is far cheaper than re-deriving it after the next surface omits a different field. The verifier fixture gap is a live blind spot today.
- **Render parity task included** because t001 may surface a read-surface change (deriving `serviceDetails.runtime`/`env` for a prebuilt-image web service) that must stay consistent across REST/GraphQL/MCP; if the matrix shows no surface change is warranted, t004 records that and the parity task degenerates to a verification-only pass.
