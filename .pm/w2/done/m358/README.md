# w2 · m358 — Sandboxes start from imported images, as on Render

**Worker:** worker2 **Goal:** `render sandbox-groups images import|get` and `render sandboxes create --image` work against bex as they do on Render. An OCI image is imported into a sandbox group, a sandbox starts from it by id or by name, and a private registry credential is used for that import only and never stored. **Status:** done

## Tasks (in order)

| id   | title | est | depends_on |
| ---- | ----- | --- | ---------- |
| t001 | Contract and design: the sandbox-build schema, `sbb-` ids, dedupe, 409/429; amend ADR014 D2 and ADR104 gap 13; where imported images live, platform, size cap, storage accounting — **DONE** | 60m | — |
| t002 | Store: a `sandbox_builds` table, unique on (group, name, digest) — **DONE** | 45m | t001 |
| t003 | Import worker: resolve the tag to a digest, copy into the platform registry, check platform and size; the credential lives in memory for one import; bounded pending builds per group — **DONE** | 2h | t002 |
| t004 | REST, GraphQL and MCP: create, list and get sandbox builds; the routes leave `NotOfferedRoutes` — **DONE** | 60m | t003 |
| t005 | Create resolves `buildId`/`buildName` to an image and starts the sandbox from it; suspend/resume and exec keep working — **DONE** | 90m | t004 |
| t006 | ADR018 sandbox rows and `docs/cli-compatibility-checklist.md` record images as served — **DONE** | 20m | t005 |
| t007 | Production walk with the pinned CLI v2.36: import, create, exec, a re-import and a private-registry import — **DONE** | 45m | t006 |
| t008 | Render parity — **DONE** | 20m | t007 |
| t009 | Simplify — **DONE** | 15m | t008 |
| t010 | Test coverage — **DONE** | 30m | t009 |
| t011 | Closeout — **DONE** | 10m | t010 |

## Definition of done

- On production, with the unmodified pinned Render CLI v2.36:
  - `render sandbox-groups images import --name py python:3.13 --wait` ends `succeeded`.
  - `render sandboxes create --image py --wait`, then `render sandboxes exec <id> -- python3 --version`, prints Python 3.13.
  - Importing the same name and tag again returns the same image instead of starting another import.
  - `images get` of an image that belongs to another group answers not found.
- An import from a private registry with `registryCredential` succeeds. Neither the database nor bex-api's logs contain the credential afterwards.
- An image that cannot be pulled, or is not `linux/amd64`, or is over the size cap, ends `failed` with a reason that says which.
- A sandbox started from an imported image idles into `suspended` and resumes on the next exec with its files, as a base-image sandbox does (w2/m278).
- `SANDBOX_IMAGES_UNSUPPORTED` is gone from the code. ADR014 D2, ADR104 gap 13 and the ADR018 sandbox rows describe images as served.

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w2` 2026-11-12, item 2, filed by `/pmx them all for w2`.
- **Render:**
  - The pinned OpenAPI (`lego/backend/internal/api/openapi/render-public-api-1.json`, ADR018 round-6 drift, 2026-11-08) has `create-sandbox-build`, `list-sandbox-builds` and `retrieve-sandbox-build` under `/sandbox-groups/{sandboxGroupId}/builds`.
  - Create takes `name` (1–255) and `sourceImage` (tag or digest), plus an optional `registryCredential {username, password}` that is "used for this build only and never stored". It "returns the build without waiting"; a request for a name and digest the group already has or is building returns that build. It answers 409 `sandbox_group_suspended`, and 429 once the group has as many builds pending as allowed.
  - CLI v2.36 (`render-oss/cli` `cmd/sandboximagesimport.go`, `cmd/sandboximagesget.go`, `cmd/sandboxcreate.go:207`): `--image` takes an image id or name, and "a name uses the newest succeeded image with that name in the sandbox group". `--build` is its deprecated alias, and both conflict with `--from`/`--snapshot-id`. An import uses the workspace's default group unless `--group` is given.
- **bex today:**
  - `lego/backend/internal/sandbox/service.go:399-414` refuses `buildId`/`buildName`/`image` with `SANDBOX_IMAGES_UNSUPPORTED`.
  - `lego/backend/internal/sandbox/notoffered.go` mounts the three builds routes only to refuse them.
  - ADR014 D2 starts sandboxes only from pre-registered templates. ADR104 gap 13 deferred images "after the REST schema is confirmed", and round 6 confirmed it.
- **Goal linkage:** ADR008 Render parity first and pillar 5 (reopened 2026-07-27). ADR014 D2 conflicts with parity, so this milestone amends it rather than keeping the difference (AGENTS.md rule).
- **Expected outcome:** an agent or customer bakes its toolchain into an image once and starts sandboxes from it, instead of reinstalling it after every create.
- **Why now:** Render Sandboxes entered public beta on 2026-11-11. The CLI bex pins already ships these commands, and today they answer bex's named 404. w2/m359 (group delete) builds on this milestone so a deleted group also deletes its images.
- **Open decision, recommendation only (not a user decision):** storage charges for imported images. Render documents none. The recommendation, under ADR008 Render parity first, is no separate charge. Imported images count in the workspace's sandbox-storage gauge (w22/m313), and a per-group image cap bounds the cost. t001 implements it and records it in ADR104. The user may sign off on a different policy.
- **Render parity included:** sandbox images on REST, GraphQL, MCP and the CLI. The dashboard's sandbox-groups pages are checked in t008.
