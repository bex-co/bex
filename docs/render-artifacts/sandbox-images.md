# Render sandbox images (sandbox builds): the contract bex serves

Captured 2026-11-12 for w2/m358 from the pinned Render OpenAPI (`lego/backend/internal/api/openapi/render-public-api-1.json`, round-6 drift of 2026-11-08) and the pinned Render CLI v2.36 (`render-oss/cli` at `4fd34231b637`). Render's public docs do not describe imported images yet (render.com/docs/sandboxes-creating and /docs/sandboxes, read 2026-11-12), so these two are the contract. The design bex chose is [ADR104 § Images](../ADR104-sandbox-render-parity.md#images-w2m358).

## Operations

| Operation | Route | Success |
| --- | --- | --- |
| `create-sandbox-build` | `POST /v1/sandbox-groups/{sandboxGroupId}/builds` | `201` build |
| `list-sandbox-builds` | `GET /v1/sandbox-groups/{sandboxGroupId}/builds` | `200` `[{build, cursor}]`, newest first |
| `retrieve-sandbox-build` | `GET /v1/sandbox-groups/{sandboxGroupId}/builds/{buildId}` | `200` build |

### Create request

| Field | Type | Notes |
| --- | --- | --- |
| `name` | string, 1–255, required | "Names the image. Every build of an image shares it." |
| `sourceImage` | string, required | "The image to convert, by tag or digest." Example `python:3.13`. |
| `registryCredential` | `{username, password}`, optional | "Pulls `sourceImage` from a private registry. Used for this build only and never stored." |

Create "starts converting an OCI image into a sandbox image, and returns the build without waiting for it. Poll the build until it `succeeded` or `failed`, then pass its ID as `buildId` to create a sandbox. A request for a name and image digest the group already has, or is building, returns that build."

### Build

| Field | Type | Notes |
| --- | --- | --- |
| `id` | string, `^sbb-[0-9a-v]{20}$` |  |
| `sandboxGroupId` | string, `^sbg-[0-9a-v]{20}$` |  |
| `name` | string |  |
| `source` | `{repository, image, reference}` | The CLI prints `repository/image:reference`, or `@reference` when the reference is a digest. |
| `sourceDigest` | string | "The image digest the build converts." Required, so the reference resolves before create answers. |
| `status` | `pending` · `running` · `succeeded` · `failed` |  |
| `failureReason` | string, nullable | "Null unless `failed`." |
| `createdAt` | date-time |  |
| `finishedAt` | date-time, nullable | "When the build succeeded or failed." |

### List query

`cursor`, `limit` (1–100, default 20), `name` ("Return only builds of the image with this name"), `status` (array of the four statuses).

### Errors

- Create: `400 invalid_sandbox_group_id` for a malformed group id, or a message when "the image is invalid or its registry can't serve it"; `409 sandbox_group_suspended`; `429` when "the group already has as many builds pending or running as its limit allows. The message includes the limit."
- List: `400 invalid_sandbox_group_id`, `invalid_status`, `invalid_cursor`, `invalid_limit`.
- Retrieve: `404 build_not_found` when the build "doesn't exist or belongs to another sandbox group"; `400 invalid_sandbox_group_id`, or `invalid build ID` for a malformed `sbb-` id.

## CLI v2.36

- `render sandbox-groups images import <source> --name <n> [--group <sbg>] [--registry-username <u> --registry-password-stdin] [--wait]`. "Uses the active workspace's default sandbox group unless --group is provided. Registry credentials are used for this import only and never stored." `--wait` polls until `succeeded`.
- `render sandbox-groups images get <sbb-id> [--group <sbg>]`. "An image that belongs to another sandbox group is reported as not found."
- `render sandboxes create --image <IMAGE_ID_OR_NAME>`: "Start from an imported image instead of the base image, by image ID or name. A name uses the newest succeeded image with that name in the sandbox group." An `sbb-` value is sent as `buildId`, anything else as `buildName`. `--build` is a deprecated alias; `--image` conflicts with `--from` and `--snapshot-id`.

## What bex answers where Render is silent

- `sandbox_group_suspended` is never produced: bex has no suspended sandbox groups.
- `invalid_cursor` and `invalid_limit`: the build list pages by the repository's policy (`core.PageParams`), as the group list does; an unknown cursor returns an empty page and a limit outside 1–100 is clamped (the repo-wide question is w22/004).
- A refused image (no linux/amd64 image, over the size cap) is a `400 SANDBOX_IMAGE_INVALID` with no build; an unreachable source is `400 SANDBOX_IMAGE_UNAVAILABLE`; the in-flight limit is `429 SANDBOX_BUILD_LIMIT`.
