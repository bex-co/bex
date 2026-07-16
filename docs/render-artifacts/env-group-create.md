# Render environment-group create contract

Captured 2026-07-15 from Render's public OpenAPI document: [`render-public-api-1.json`](https://api-docs.render.com/openapi/render-public-api-1.json), operation `POST /env-groups` (`create-env-group`). This file pins only the create contract needed by w5/m33; the repository's small `lego/backend/internal/api/testdata/render-openapi.json` compatibility fixture does not contain the environment-group paths.

## Request

The request schema is `envGroupPOSTInput`:

| field | required | shape |
| --- | --- | --- |
| `name` | yes | string |
| `ownerId` | yes | string |
| `envVars` | yes | array of `{key, value}` or `{key, generateValue}` objects |
| `secretFiles` | no | array of `{name, content}` objects |
| `serviceIds` | no | array of service-id strings to link at creation |
| `environmentId` | no | string |

Each env-var entry is a `oneOf`:

- literal: required string `key` and string `value`;
- generated: required string `key` and boolean `generateValue`; Render documents `generateValue: true` as mutually exclusive with `value`.

Each secret file requires both string fields `name` and `content`.

## Success and invalid service ids

Success is `201 Created` with an `envGroup` response. The response combines `envGroupMeta` (`id`, `name`, `ownerId`, `createdAt`, `updatedAt`, `serviceLinks`, optional `environmentId`) with `envVars` and `secretFiles`.

The operation declares `404 Not Found` for a referenced resource that cannot be found, including an invalid linked `serviceId`. It uses Render's standard `error` JSON object with string `id` and `message` fields. The OpenAPI document does not pin stable values for either string, so clients must use the HTTP status and envelope rather than matching prose. bex returns the same status/envelope and names the rejected `serviceId` in `message`.

The public OpenAPI does not specify partial-write behavior or expose a way to observe an orphan after a failed create. bex therefore adopts the milestone's stronger explicit guarantee: it validates every env var, file, Environment, and service link before minting state, and compensates later store/Secret/App writes if persistence fails. A validation failure leaves no group, projection Secret, or App-spec change.
