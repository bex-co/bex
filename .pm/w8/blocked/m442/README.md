# w8 · m442 — Render contract round 8: logs take a `build` filter, an env group read omits the values a caller cannot see, and a sandbox build name cannot start with `sbb-`

**Worker:** worker8 **Goal:** bex answers the three operations Render changed in its public API on or before 2026-11-16 as Render now does, and the pinned spec matches Render's again, so the daily drift check is green. **Status:** blocked (`w22/m444`: t001–t007 are done, shipped in `9c58dd1ac` and live since pin `55c120858af0`. The logs, sandbox-build and spec parts of the definition of done passed live on 2026-11-17 (decision record below). t008 waits for the drift job's CLI check, which needs `w22/m444`'s render-oss/cli v2.37.0 pin, and then the env-group viewer read)

## Tasks (in order)

| id   | title | est | depends_on |
| ---- | ----- | --- | ---------- |
| t001 | `GET /logs`, `/logs/subscribe` and `/logs/values` accept Render's `build` filter, and build lines carry a `build` label — **DONE** | 1h | — |
| t002 | `GET /env-groups/{id}` serves a caller without the secret-read permission the group with `envVars` and `secretFiles` omitted, not a refusal — **DONE** | 45m | — |
| t003 | A sandbox build `name` starting with `sbb-` is refused, as Render refuses it — **DONE** | 15m | — |
| t004 | Re-pin `render-public-api-1.json` to Render's 2026-11-16 spec and record round 8 in ADR018 — **DONE** | 20m | t001, t002, t003 |
| t005 | Render parity — **DONE** | 20m | t004 |
| t006 | Simplify — **DONE** | 15m | t005 |
| t007 | Test coverage — **DONE** | 30m | t005 |
| t008 | Closeout | 10m | t007 |

## Definition of done

- `scripts/render-schema-drift.sh` passes against `https://api-docs.render.com/openapi/render-public-api-1.json`, and the scheduled `render-schema-drift.yml` run is green.
- **Logs:**
  - `GET /v1/logs?ownerId=<tea>&resource=<srv>&build=<id>&limit=5` answers 200 with only that build's lines, and an unknown build answers an empty page.
  - `/logs/subscribe` with `build` streams only that build.
  - `/logs/values?label=instance&build=<id>` lists only that build's instances. `label=build` stays Render's 400: Render's 2026-11-16 values `label` enum did not gain `build`, which the filing misread (amended by w8/m442 t001, 2026-11-16).
  - On production pin `7aaa8700befa` all three answer 400 (`unsupported query parameter "build" for list-logs`; `label must be one of instance|host|statusCode|method|level|type`).
- **Env groups:**
  - A workspace member without `can_view_sensitive` reads a group and gets 200 with its metadata, with `envVars` and `secretFiles` absent.
  - A member with the permission still gets the values.
  - No value is revealed or audited as revealed on the omitted path.
- **Sandbox builds:** `POST /v1/sandbox-groups/{id}/builds` with `name: "sbb-anything"` is refused with Render's 400 code, and a name like `my-sbb-image` is still accepted.

## Source + Goal linkage

- **Source:** `/qa-find-bugs-cli` sweep `f50c` (2026-11-16 18:20 UTC). The `render-schema-drift.yml` run at 15:59 UTC failed with "operations bex serves changed inside their schemas upstream", and nothing on the board picked it up. A field-level diff of the pinned spec (vendored at `d8c71580b`, round 7, `w3/m367`) against the live one leaves 8 substantive differences; the other 70 are rewrapped descriptions:
  - **`envGroup`:** `envVars` and `secretFiles` lost `required`, and each now reads "Omitted if the caller doesn't have the permission to read the environment group's secret values."
  - **`/logs`:** gains query parameter `build`: `{"in":"query","required":false,"description":"Filter logs by build ID, such as a service build or a sandbox image build.","schema":{"type":"array","items":{"type":"string"}}}`.
  - **`/logs/subscribe` and `/logs/values`:** gain the same parameter (by `$ref`).
  - **The `/logs/subscribe` 101 label-name enum:** gains `build`.
  - **The sandbox build create `name`:** now reads "Can't start with `sbb-`, the prefix reserved for build IDs."
- **Goal linkage:** ADR008 § Render parity first. ADR018's Render-contract rounds keep bex's REST surface on Render's current contract (round 7 was `w3/m367`).
- **Expected outcome:**
  - A Render client that starts sending `build` (the next Render CLI with a logs `--build` flag, or the SDKs) works on bex instead of getting 400.
  - A developer-role member can open an env group's metadata, as on Render.
  - The drift alarm is green again, so the next drift is noticed.
- **Why now:** the drift job alarms daily, and while it is red a further upstream change would land unnoticed. The `build` filter is the one most likely to break a client soon: the CLI is pinned to upstream v2.36.0, and a pin bump that brings a `--build` flag would ship a flag that 400s on bex.

### bex today (production pin `7aaa8700befa`, 2026-11-16 18:30 UTC, QA user, bex-canary)

```text
GET /v1/logs?ownerId=<tea>&resource=<srv>&type=build&limit=5            → 200, 5 build lines
GET /v1/logs?ownerId=<tea>&resource=<srv>&build=bld-nonexistent…&limit=5 → 400 {"error":"unsupported query parameter \"build\" for list-logs","id":"bad_request",…}
GET /v1/logs/values?ownerId=<tea>&resource=<srv>&label=build             → 400 {"error":"label must be one of instance|host|statusCode|method|level|type",…}
```

- **Env groups:** `lego/backend/internal/envgroups/render.go:130-150` (`revealGroups`) authorizes every REST read for `can_view_sensitive` and "a caller without it is refused the whole read rather than served a masked body". That is not run live: the QA user has the permission.
- **Sandbox builds:** `lego/backend/internal/sandbox/images.go:381-388` (`validateImageName`) checks only 1–255 characters, no surrounding spaces and no control characters, so a `sbb-` name is accepted. That is not run live: sandbox builds belong to the w32 loop's fixtures.

### Unverified

- **The ID a "service build" carries on Render.** The spec's only build ID pattern is the sandbox one (`^sbb-[0-9a-v]{20}$`). bex's `bld-` kind is a Blueprint build-source build (`internal/id/id.go:157`), and a service's build lines are keyed by deploy today. t001 must decide which ID a service build's `build` label carries, and record the choice in ADR018 as bex's choice until Render's value is observed.
- **Which roles lack the secret-read permission on Render** (the spec says only "the permission"), and whether GraphQL and MCP env-group reads should mirror the REST omission. That is t005's question.
- **Render's exact 400 code for a reserved `sbb-` name.**

### Resolved by t001–t005 (2026-11-16)

- **A service build's ID:** its deploy's `dep-` id, as bex's choice until Render's value is observed (ADR018 round 8, Build logs row).
- **Roles without the secret-read permission in bex:** viewer, contributor and billing (`can_view_sensitive: developer or admin`). GraphQL and MCP group reads already answered them with names and no values, so they keep the names (ADR018 Environment groups row).
- **Render's 400 code for an `sbb-` name:** none. Render's create documents a message only, so bex answers `SANDBOX_IMAGE_INVALID` with a message naming the prefix.
- **Upstream moved again after filing:** a Workflows step-attempt read (`getFlowRunStepAttempt`), now an omission, and render-oss/cli v2.37.0. The CLI bump is `w22/m444`, and the drift job stays red on its CLI check until that lands.


## Decision record — 2026-11-17: release gate cleared, live check passed except the drift job's CLI check

- **Affected:** w8/m442 (t008); gate owner `w22/m444`.
- **Source:** verified evidence, `/qa-find-bugs-cli` sweep `6451` (08:10–08:20 UTC). Production pin `55c120858af0` was rolled at 07:47 UTC by `9c79201b4`, and `git merge-base --is-ancestor 9c58dd1ac 55c120858af0…` holds. bex CLI v0.7.0, QA user, `bex-canary`.
- **Logs (passed):** fixture static site `srv-dbu0nqlikg6s738lalgg` (deleted). Its two builds are D1 `dep-dbu0nqlikg6s738lalh0` and D2 `dep-dbu0pdec1l2s739d3mvg`, 9 build lines each.

  ```text
  GET /v1/logs?ownerId=<tea>&resource=<srv>&build=D1&limit=50                → 200, 9 lines, all build=D1
  GET /v1/logs?…&build=D2&limit=50                                            → 200, 9 lines, all build=D2
  GET /v1/logs?…&build=D1&build=D2&limit=50                                   → 200, 18 lines (9 + 9), = type=build
  GET /v1/logs?…&build=dep-zzzzzzzzzzzzzzzzzzzz&limit=20                     → 200, 0 lines, hasMore false
  GET /v1/logs/values?…&label=instance&build=D1                               → 200 ["srv-dbu0nqlikg6s738lalgg-6v83bsmf6gt6sqmh004j"]
  GET /v1/logs/values?…&label=type&build=D1                                   → 200 ["build"]
  GET /v1/logs/values?…&label=level&build=dep-zzzzzzzzzzzzzzzzzzzz           → 200 []
  GET /v1/logs/values?…&label=build                                           → 400 "label must be one of instance|host|statusCode|method|level|type"
  WS  /v1/logs/subscribe?…&build=D2, opened as D2 started (180 s)             → 101; 9 lines, all build=D2, the build-command marker among them
  WS  /v1/logs/subscribe?…&build=D1, same window (D1 finished)                → 101; 0 lines
  ```

- **Sandbox builds (passed):** QA group `sbg-dbu0nulikg6s738lalig` (deleted with its builds).
  - `bex sandbox-groups images import docker.io/library/busybox:1.37 --group <grp> --name sbb-qa6451` → exit 1 in 0.3 s: `received response code 400 (SANDBOX_IMAGE_INVALID): name can't start with "sbb-", the prefix reserved for build IDs`.
  - `--name my-sbb-qa6451` and `--name SBB-qa6451` were accepted (`sbb-dbu0o1ec1l2s739d3m3g`, `sbb-dbu0nutikg6s738lalk0`).
  - t008's count: builds named `sbb-…` in `bex-canary` = **0**, in the shared Default group `sbg-dbpugr7bq3ic73c0acmg` and the QA group alike. No wider production read is reachable from the QA user.
- **Spec drift (passed for the spec):** `bash scripts/render-schema-drift.sh` at 08:19 UTC reports:
  - the Blueprint schema matches pinned `6c6721a6…`;
  - the 68-value webhook enum matches;
  - all 264 REST operations match;
  - the schemas of the 211 operations bex serves match.

  It exits 1 only on `render-oss/cli v2.37.0 is newer than bex's pin v2.36.0`.
- **Env groups (not verified live):** the QA user holds `can_view_sensitive`. A viewer, contributor or billing member's read (values omitted, nothing audited as revealed) needs such a member's credential, which this hunt does not have.
- **Remaining gate:** `w22/m444` (render-oss/cli v2.37.0 pin, `todo` on 2026-11-17). Release condition: the scheduled `render-schema-drift.yml` run is green after m444 ships.
- **Next steps for t008:**
  1. Once `gh run list --workflow render-schema-drift.yml --limit 1` reads `success`, run the env-group viewer read with a workspace member who lacks `can_view_sensitive`: `GET /v1/env-groups/<id>` → 200 without `envVars`/`secretFiles`; an admin's read still lists them.
  2. Then close the milestone.
