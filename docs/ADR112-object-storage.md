# ADR112 — bex object storage: Render's `ea objects` and Cloudflare's `wrangler r2` contracts over Wasabi

**Status:** proposed (2026-11-14). Filed 2026-11-15 as [w21/m415](../.pm/w21/blocked/m415/README.md), [m416](../.pm/w21/m416/README.md) and [m417](../.pm/w21/m417/README.md); § Tasks maps the split.

**Source:** user request 2026-11-14: an ADR to support Render's early-access object store with Wasabi S3. After a study of what customers need from object storage, the user restated the goal: "rw cli contract + wasabi wrapper", and asked for the ADR to be rewritten around it. I read "rw" as **R**ender and **w**rangler: serve the contract of Render's CLI (`render ea objects`) and of Cloudflare's `wrangler r2`, each as a thin wrapper over one Wasabi-backed store. If only wrangler was meant, D2 and the `render-<region>` system buckets in D1 drop out and nothing else changes.

**Rewritten 2026-11-14.** The first version served only Render's four routes. Render's contract has no bucket, no public access, no CORS and no lifecycle, so it is too thin to serve what customers need. `wrangler r2` has all four, and it can be pointed at another API with three environment variables, so bex adds no CLI of its own.

**Amended 2026-11-15.** Tenant data shares the platform's existing Wasabi account instead of a dedicated one (user decision, D1), so there is no second account to create and no new root credential. D9 now names the identities and the four credential variables, and fixes the rule that environment variables carry credentials only: endpoints, buckets, regions and caps are code constants, and the objectgw kill switch is a scale to zero (D4).

**Supersedes:** the "Object storage (`render ea objects`, early access)" row of [ADR018](ADR018-render-parity.md), a signed divergence recorded on 2026-10-29 by w6/097 option B ([the triage note](../.pm/w6/done/097.md)). The row's own re-open condition was "Render takes the feature GA or a migration needs a staging bucket". This ADR re-opens it on the user's instruction.

**Extends:** [ADR008 § Render parity first](ADR008-vision.md#render-parity-first), [ADR006](ADR006-bex-api.md) (one core, thin adapters), [ADR095](ADR095-agent-attachments.md) (bucket, IAM and presign pattern), [ADR074](ADR074-workspace-scoped-artifact-identity.md) (workspace-scoped S3 prefixes), [ADR029](ADR029-static-sites.md) and [ADR103](ADR103-security-review-static-rewrite-proxy.md) (a read-only object-store proxy and its threat model), [ADR093](ADR093-onbex-wildcard-tls.md) (the wildcard certificate), [ADR020](ADR020-identifiers.md) (ids), [ADR050](ADR050-encrypted-platform-backups.md) and [ADR019](ADR019-infra-credentials.md) (credentials), [ADR023](ADR023-usage-metering.md), [ADR030](ADR030-pricing.md) and [ADR040](ADR040-billing-metronome.md) (metering, price and Stripe), [ADR012](ADR012-auth.md) and [ADR024](ADR024-members.md) (scopes and roles), [ADR086](ADR086-account-deletion.md) (cleanup).

**Amends when accepted:** ADR018 (the object storage row, a new `Extension` row for the Cloudflare-shaped API, and dated divergences from D1, D3, D4 and D8), [cli-compatibility-checklist.md](cli-compatibility-checklist.md) (the four `ea objects` rows), [bex-cli.md](bex-cli.md) (a wrangler section), `lego/backend/AGENTS.md` (the environment inventory), ADR019 (two credential rows).

## Context

### What Render publishes

Verified on 2026-11-14. There is **no design document, RFC, blog post or changelog entry** about object storage. What exists:

- **Feedback board.** The 2019-07-16 request asks for "the capabilities of AWS S3" through S3-compatible APIs. Staff posted "Object Storage is now available in alpha" on 2026-03-01 and pinned "sign up for the waitlist" on 2026-03-09. The request is in progress with 467 votes. Nothing there discusses buckets, credentials, price or regions. The page's "Load More" comments were not read.
- **`render-oss/sdk`.** Its README lists Object Storage as "Soon" for the REST API and "Early Access client" for Python and TypeScript, both under `experimental`. The REST API is not yet officially public, and the public OpenAPI pinned in the repo has no `/objects` path (w6/097).
- **`render-oss/cli`** releases v2.27.0 to v2.36.0 carry no object storage entry. The one graduation precedent is v2.32.0, which moved `render ea sandboxes` to `render sandboxes`. `render ea` is documented as "subject to change".
- **Not found anywhere public:** regions, price, quotas, presigned-URL lifetime, maximum object size, key rules, delete-missing status, overwrite semantics, and any statement about buckets or public access. A search summary attributed "private by default… signed URLs or public access controls" to `render.com/object-storage`; the page itself fetched as navigation only, so it is not evidence. Per the lesson recorded in memory (t012 shipped 100 s where Render says 100 min), none of the unknowns is quoted below as Render's number.

### Contract A: what the pinned Render CLI and the SDKs enforce

The sources are `render-oss/cli@v1.1.3-0.20261106232710-4fd34231b637` (`pkg/storage/repo.go`, `pkg/client/storage/storage_gen.go`, `cmd/object*.go`) and the Python and TypeScript object clients in `render-oss/sdk`. All call the same four routes:

| Route | Request | Response |
| --- | --- | --- |
| `GET /v1/objects/{ownerId}/{region}` | `cursor`, `limit` | `{items:[{cursor, object:{key, sizeBytes, lastModified}}], hasNext, nextCursor?}` |
| `PUT /v1/objects/{ownerId}/{region}/{key}` | `{sizeBytes}` | `{url, expiresAt, maxSizeBytes}` |
| `GET /v1/objects/{ownerId}/{region}/{key}` | none | `{url, expiresAt}`; the CLI maps 404 to "object not found" |
| `DELETE /v1/objects/{ownerId}/{region}/{key}` | none | 200 or 204; the CLI maps 404 to "object not found" |

- **There is no bucket in the contract.** `ownerId` is the workspace id (`tea-…`) and `--region` is mandatory, so a tenant sees one namespace per workspace per region.
- The client PUTs or GETs the returned URL itself. A zero-byte upload sends `http.NoBody`, because "presigned S3 URLs do not support chunked encoding" (`repo.go`).
- **`maxSizeBytes` equals the requested `sizeBytes`.** Both SDKs refuse an upload whose size differs from the response's `maxSizeBytes`, so the URL signs that exact length. They send the caller's `Content-Type` and `Content-Length`, read the response's `ETag`, and on a GET read `size` and `content-type`.
- `ea objects list` reads the first page only, with `--limit` defaulting to 100; its help example uses `--limit=200`. Python's `list` docstring gives a default of 20. The generated `Region` type lists `frankfurt`, `ohio`, `oregon`, `singapore`, `virginia`.

### Contract B: what `wrangler r2` calls

Measured on 2026-11-14 from the wrangler source (`cloudflare/workers-sdk`, `packages/wrangler/src/r2/**`) and by running stock **wrangler 4.162.0** against a throwaway fake of these routes. The fake lived in the session scratchpad and is not part of the repository.

| `wrangler r2 …` | Request | What wrangler needs back |
| --- | --- | --- |
| `bucket list` | `GET /accounts/{id}/r2/buckets` | `result.buckets[{name, creation_date, location?, storage_class?}]` |
| `bucket create <name>` | `POST …/buckets`, body `{name, locationHint?, storageClass?}` sent as `text/plain`, header `cf-r2-jurisdiction` | `result{name, creation_date, location, storage_class, jurisdiction}` |
| `bucket info <name>` | `GET …/buckets/{b}`, then `POST /graphql` (`getR2StorageMetrics`) | the bucket; the metrics are optional (unanswered, wrangler printed `object_count: 0`) |
| `bucket delete <name>` | `DELETE …/buckets/{b}` | envelope |
| `object put <b>/<key>` | `PUT …/buckets/{b}/objects/{key}`, the bytes, `Content-Length`, optional `Content-Type`, `Cache-Control`, `Content-Disposition`, `Content-Encoding`, `Content-Language`, `Expires`, `cf-r2-storage-class`, `cf-r2-data-catalog-check: true` | a 2xx **with a body** (source: `fetchR2Objects` treats a bodiless 2xx as failure) |
| `object get <b>/<key>` | `GET …/objects/{key}` | the raw bytes; 404 means missing |
| `object delete <b>/<key>` | `DELETE …/objects/{key}` | a 2xx with a body |
| `bucket dev-url get\|enable\|disable` | `GET\|PUT …/{b}/domains/managed`, `PUT` body `{enabled}` | `result{bucketId, domain, enabled}` |
| `bucket domain list\|add\|update\|remove` | `GET\|POST\|PUT\|DELETE …/{b}/domains/custom[/{domain}]` | `{domain, enabled, status{ownership, ssl}, minTLS, zoneId, zoneName}` |
| `bucket cors list\|set\|delete` | `GET\|PUT\|DELETE …/{b}/cors`, body `{rules:[{allowed:{origins, methods, headers}, exposeHeaders, maxAgeSeconds}]}` | `result{rules}` |
| `bucket lifecycle list\|add\|set\|remove` | `GET\|PUT …/{b}/lifecycle`, rules `{id, enabled, conditions{prefix}, deleteObjectsTransition{condition{type: Age\|Date, maxAge\|date}}, storageClassTransitions, abortMultipartUploadsTransition}` | `result{rules}` |

Facts that shape the design:

- **Pointing wrangler elsewhere takes three variables:** `CLOUDFLARE_API_BASE_URL`, `CLOUDFLARE_API_TOKEN` and `CLOUDFLARE_ACCOUNT_ID`. wrangler accepts account ids matching `^[A-Za-z0-9_-]+$` (`workers-auth/src/account-id.ts`), so `tea-…` is valid. With the three set, the run showed **no** `/memberships`, `/accounts` or token-verify request, only the routes above.
- **Every JSON answer is Cloudflare's envelope** `{success, errors:[{code, message}], messages, result}`. Object GET answers raw bytes.
- **The bytes go through the API.** `object put` sends the file as the request body. wrangler refuses files over 300 MiB (`MAX_UPLOAD_SIZE_BYTES`, "the maximum file size we can upload using the V4 API"). There is no presigned URL in this contract.
- **`object put|get|delete` default to local storage** (`isLocal` defaults to true). A customer must pass `--remote`. All my runs used it.
- **wrangler has no object list command.** `r2 object` offers `get`, `put`, `delete` and `bulk put`. Listing a bucket is an S3 API job.
- **What I ran:** `bucket list|create|info|delete`, `object put|get|delete --remote`, `dev-url get|enable -y`, `cors list`, `lifecycle list` and `domain list`. **What I did not run:** `dev-url disable`, `cors set`, `lifecycle add|set|remove`, `domain add|update|remove`, `object bulk put`, and `bucket create` with `--location`, `--jurisdiction` or `--storage-class`. Their shapes come from source only.

### What bex has today

- The four Render routes answer 404 `bex does not offer Render's early-access object storage yet` (`lego/backend/internal/api/server.go:1749-1777`, pinned by `object_storage_not_offered_test.go`).
- The launcher refuses `ea objects` before sending anything, and `--local` still works (`lego/cli/object_storage_test.go`).
- The scope matrix classifies the four routes as read or write (`lego/backend/internal/api/scope_matrix_ops.go:682,799,800,1033`).
- `docs/cli-compatibility-checklist.md:324-332` marks the four commands `[-]`.
- bex already ships bex-only surfaces beside Render's: ADR018 counts 234 `Extension` MCP tools, and the `bex` CLI registers its own `bex code`, `bex edit` and `bex upgrade` ([bex-cli.md](bex-cli.md)). A surface Render lacks is an extension, not a parity break, as long as Render's own routes keep their contract.

### What bex already has for Wasabi and for serving it

- `lego/backend/internal/s3client` builds a path-style client, which Wasabi needs. `lego/backend/internal/postgres/export_signer.go` is the presign precedent, and `lego/backend/internal/apps/disk_snapshots.go` pages `ListObjectsV2`.
- [ADR095](ADR095-agent-attachments.md) D1 and D7 set the pattern: one bucket, one bucket-scoped IAM user, a 15-minute presigned GET. `infra/wasabi/agent-attachments-s3-policy.json` and `scripts/agent-attachments-s3-credentials.sh` provision, verify and install it.
- [ADR029](ADR029-static-sites.md)'s static-server is the serving precedent: one always-on proxy with a **read-only** identity, signed GETs against a private bucket, an in-memory cache, and tenant content on `*.onbex.co` under the wildcard certificate. [ADR093](ADR093-onbex-wildcard-tls.md) amended 2026-11-14: a single label under `BEX_BASE_DOMAIN` gets no per-host certificate.
- bex-api's `http.Server` has `ReadTimeout` and `WriteTimeout` of 30 s (`lego/backend/cmd/api/main.go`, `newHTTPServer`; `api.HTTPWriteTimeout`, `internal/api/graphql_cost.go:30`). `internal/deploys/platform_rebuild.go:192` already extends a deadline per request with `http.NewResponseController`. Traefik's `readTimeout` is 6000 s (`deploy/gitops/base/values/traefik.values.yaml:72`).
- The production Wasabi account already serves `eu-central-2` (Frankfurt). `lego/backend/internal/resourcemeta/metadata.go:133-141` maps `fsn1` to `frankfurt` and lists Render's five regions.

### Wasabi facts (read 2026-11-14)

- Pricing page: "Starting at $7.99 TB/month", a 1 TB minimum billed per account, a 90-day minimum storage duration, and no egress or API fees. It does not say whether the 90 days applies to deleted objects.
- The pricing FAQ conditions free egress on monthly downloads staying at or below the active storage volume, and reserves the right to limit or suspend a service that exceeds that regularly.
- A default of 1000 buckets per account (a soft limit raised through support). Service URLs follow `https://s3.<region>.wasabisys.com`; `us-west-1` is Oregon and `eu-central-2` is Frankfurt.
- Wasabi encrypts every object at rest with AES-256 but does not implement `Put/GetBucketEncryption` (`.pm/w2/done/m77/done/t001.md`).

## Implementation at a glance

![Architecture: render ea objects reaches bex-api's Render routes, which authorize and sign a presigned URL so the bytes move client to Wasabi directly; stock wrangler r2 reaches bex-api's Cloudflare-shaped routes, and the bytes stream through bex-api; buckets are rows in Postgres over one shared Wasabi bucket per region; a public bucket is served by objectgw at pub-id.onbex.co; an hourly sweep feeds quotas, lifecycle rules and shadow meters; the platform's backups sit in other buckets of the same Wasabi account, under other credentials](assets/ADR112-object-storage.svg)

Solid arrows are requests, dashed arrows are answers, grey is unchanged, and amber marks limits bex chose. The picture is the design, not the state of the code: nothing in it is implemented yet.

## Decision

### D1 — One store: virtual buckets over one shared Wasabi bucket per region

- **Layout.** A bucket per region on Wasabi, `bex-objects-<region>` (final names are settled at provisioning, since Wasabi bucket names are global). The object key is `<tea-id>/<obk-id>/<key>`, the prefix idea of [ADR074](ADR074-workspace-scoped-artifact-identity.md).
- **Buckets are rows.** `object_buckets(id, workspace_id, name, region, system, public, cors, lifecycle, created_at)` with `(workspace_id, name)` unique. `id` is a new typed id, `obk-<xid>`, minted through `internal/id` ([ADR020](ADR020-identifiers.md)). A name follows wrangler's own rule, `^[a-z0-9][a-z0-9-]{1,61}[a-z0-9]$` (`workers-utils/src/config/validation.ts`), and never changes. **The Wasabi key embeds the id, not the name**, so a bucket that is deleted and recreated under the same name never sees the old objects. A bucket must be empty to be deleted.
- **Why virtual.** Wasabi allows 1000 buckets per account by default, and a tenant never holds a Wasabi credential, so a physical bucket per customer bucket isolates nothing a key prefix does not.
- **Contract A's namespace is a system bucket.** Render's `{ownerId}/{region}` resolves to the workspace's bucket `render-<region>`, created on first use (`system = true`, outside the bucket cap, and the `render-` prefix is reserved). An object put with the Render CLI is therefore readable with wrangler and the other way round.
- **Regions at launch.** Cloudflare's location hint `wnam` and the default map to `oregon` (`us-west-1`); `weur` and the jurisdiction `eu` map to `frankfurt` (`eu-central-2`). A bucket created without a hint goes to `oregon`, a constant (D9). Every other hint or Render region (`ohio`, `virginia`, `singapore`, `enam`, `eeur`, `apac`, `oc`, `fedramp`) answers 400, recorded in ADR018 as a dated divergence. Cloudflare calls a hint "best effort and not a guarantee"; bex's region is exact, and `bucket info` reports the hint it was created with.
- **Account (user decision 2026-11-15: shared).** Tenant data goes on the platform's existing Wasabi account, the one that holds `bex-tfstate` and every backup and drilled restore path ([ADR031](ADR031-platform-data-backup.md), `.pm/FUTURE-MAYBE.md:23`), in buckets and under IAM users of its own (D9). The first draft used a separate account so that a Wasabi limit could not reach the backups; sharing gives that up. Wasabi may limit a service whose egress regularly exceeds its storage, public buckets (D4) make that likelier, and I assume Wasabi would act on the account, not on one bucket (unverified). What sharing saves is a second 1 TB minimum, about $8 a month at today's page, and a second root credential to guard. The first response to a Wasabi warning is objectgw's kill switch (D4); the second is moving tenant data to a separate account, which changes only the values in D9, not the code, and must not block D2 or D3.
- **Posture.** Private, no versioning (it would double the Wasabi bill), no CORS and no ACLs at Wasabi; CORS and public access are bex's (D4). Encryption at rest is Wasabi's automatic AES-256.

### D2 — Contract A: the four Render routes, served as the pinned CLI and SDKs call them

Remove the four entries from `notOfferedRefusals` and the `objectStorageNotOffered` constant. Add `lego/backend/internal/objectstorage`, a core service with the REST handlers as a thin adapter ([ADR006](ADR006-bex-api.md)). Errors keep Render's shape.

- **list.** The service reads Wasabi directly over `<tea-id>/<obk-id of render-region>/`, with no database mirror. `limit` is honored up to 1000, the S3 page size; the CLI never follows `nextCursor`, and its own help example asks for 200, so capping at 100 would truncate silently. The cursor is the base64 of the last key, fed to `StartAfter`. The default is 20, from the Python docstring; Render's default and cap are unverified.
- **put.** Mint a presigned PUT that signs `Content-Length` equal to `sizeBytes`, so an upload cannot exceed what the quota gate reserved, and return `maxSizeBytes` equal to `sizeBytes`, which the SDKs require. Do not sign `Content-Type`: the request carries only `sizeBytes`, the client's header is stored as sent, and the SDKs read it back. The PUT response must expose the object's `ETag`. A size above the per-object cap is `413`.
- **get and delete.** `HeadObject` first, and a missing key answers `404`: S3's `DeleteObject` answers 204 for a missing key, while the CLI treats 404 as "object not found".
- **get URL.** Pin `response-content-disposition: attachment`, as [ADR095](ADR095-agent-attachments.md) D7 does for anything that is not a raster image, so a browser opening the URL does not render tenant HTML on Wasabi's origin. Do not pin `response-content-type`.
- **Metering.** The bytes bypass bex, so egress is a shadow estimate from the size `HeadObject` returns when a GET URL is minted: exact for a complete download, an upper bound for a ranged one.

### D3 — Contract B: the Cloudflare-shaped routes `wrangler r2` calls, with the bytes streamed through bex-api

bex-api serves, under `/cloudflare/client/v4/accounts/{accountId}/r2`, exactly the routes in the Contract B table, so a customer sets:

```sh
export CLOUDFLARE_API_BASE_URL=https://api.bex.co/cloudflare/client/v4
export CLOUDFLARE_API_TOKEN=<a bex API key>     # sent as Authorization: Bearer, like every bex route
export CLOUDFLARE_ACCOUNT_ID=<tea-id>           # the workspace
wrangler r2 object put photos/cat.jpg --file cat.jpg --remote
```

The routes are not in Render's OpenAPI, so [ADR018](ADR018-render-parity.md)'s strict Render request gate (bex's mux intersected with the pinned Render spec) does not cover them; bex validates its own input. Each route gets a `REST <METHOD> <pattern>` entry in `classifiedOps` (`scope_matrix_ops.go`, regenerated by `GENERATE_SCOPE_MATRIX=1 go test ./internal/api -run TestGenerateScopeMatrix`).

- **Envelope and errors.** Every JSON answer is Cloudflare's envelope, and every 2xx carries a body, because wrangler fails on a bodiless one. Errors use Cloudflare's published codes (read 2026-11-14): `10002` Unauthorized (401), `10003` AccessDenied (403, also for a workspace the caller cannot see, so existence does not leak), `10005` InvalidBucketName (400), `10006` NoSuchBucket (404), `10007` NoSuchKey (404), `10008` BucketNotEmpty (409), `10009` TooManyBuckets (400), `10020` InvalidObjectName (400), `10033` MissingContentLength (411), `10042` NotEntitled (403, for a feature or region bex does not offer and for an exceeded quota, with a message saying which), `10058` TooManyRequests (429), `10073` BucketConflict (409, an existing name; the docs list no "already exists" code) and `100100` EntityTooLarge (400). A name is unique per workspace, and bex ignores `cf-r2-jurisdiction` when looking one up, where Cloudflare scopes names per jurisdiction.
- **Buckets.** `POST` creates a row (no Wasabi call), `GET` lists the workspace's buckets across regions, `DELETE` checks emptiness with a one-key `ListObjectsV2`. `PATCH` (storage class) and `storageClass: InfrequentAccess` answer `10042`: bex has one class.
- **Objects stream.** The handler extends the per-request read and write deadlines with `http.NewResponseController` (the 30 s server timeouts would otherwise cut a large upload), requires `Content-Length` (`10033`), refuses above 300 MiB (wrangler's own cap, a constant), runs the quota gate (D8), and streams to Wasabi. Streaming uses a server-signed presigned PUT, so no payload hash and no spooling; the stored `Content-Type`, `Cache-Control`, `Content-Disposition`, `Content-Encoding`, `Content-Language` and `Expires` are the request's. A GET `HEAD`s first (`10007` when missing), then streams the object back with its stored headers. A put overwrites. Concurrent streams are capped per workspace and per replica (`429`, `10058`). Wasabi enforces a signed `Content-Length` and refuses a chunked body (§ Not verified, settled 2026-11-15), so the handler passes the request's own `Content-Length` on and never lets the client library chunk; a body streamed from a network reader is confirmed by the Contract B tests and walk.
- **Why not a redirect to a presigned URL.** It would keep bytes off bex-api, but wrangler sends the file as a stream, and the fetch standard cannot replay a stream body after a 307 or 308. I did not test it.
- **`bucket info`.** `POST /graphql` answers `getR2StorageMetrics` from the last sweep's per-bucket usage. Unanswered, wrangler shows zeros (verified), which is wrong rather than broken.
- **Not offered, each answering `10042`:** `lock`, `notification`, `sippy`, Data Catalog, and custom domains (`domain add|update|remove`; `domain list` returns an empty list). Custom domains would reuse [ADR005](ADR005-custom-domain.md)'s machinery and are a follow-up.
- **No `bex` command is added.** wrangler is the client. The launcher's pass-through of `ea objects` (D11) is Contract A only.

### D4 — Public access: `dev-url` enables a read-only gateway at `pub-<obk-id>.onbex.co`

`PUT …/{b}/domains/managed {enabled: true}` sets `public` on the bucket and answers `{bucketId: "obk-…", domain: "pub-<obk-xid>.<BEX_BASE_DOMAIN>", enabled: true}`. `enabled: false` stops serving. Private is the default for every bucket.

- **objectgw.** A new binary in the backend image, deployed like the static-server ([ADR029](ADR029-static-sites.md)): `GET` and `HEAD` only, `Range` and conditional requests passed to Wasabi, no listing (the root answers 404), a **read-only** Wasabi identity, a cache. It resolves `Host` to a bucket by asking bex-api (cached 30 s, misses cached too), so the gateway holds no database credential. A private, deleted or unknown host answers 404.
- **Host.** `pub-<obk-xid>` is a single label under `BEX_BASE_DOMAIN`, so the existing wildcard certificate covers it with no per-host order ([ADR093](ADR093-onbex-wildcard-tls.md) w4/650). The `pub-` prefix is reserved in service-slug validation so a service cannot take a bucket's host. Routing `pub-*` hosts to objectgw is an infrastructure task (t006).
- **Trust.** Public bucket content is served from the same registrable domain as tenant apps and static sites, so it carries the same risk class: tenant-controlled HTML and script on a sibling host ([ADR103](ADR103-security-review-static-rewrite-proxy.md) is the model). objectgw sets `X-Content-Type-Options: nosniff` and serves the stored `Content-Type` and `Cache-Control`; it does not claim stronger isolation than static sites have.
- **CORS.** `cors set` stores the rules on the bucket row, and objectgw applies them, including preflight. Contract B needs a token, so a browser never reaches it, and Wasabi's CORS stays off. The bex cap is 100 rules and 64 KiB per bucket.
- **Authorization.** Enabling or disabling needs `can_manage` (workspace admin), because it publishes customer data. Reads need nothing. A system bucket (`render-<region>`) can be made public by the same call; the Render CLI has no way to know.
- **Egress is the exposure.** Wasabi's free egress is a fair-use ratio, and public reads are the case most likely to break it. objectgw counts every byte out, enforces per-bucket and per-workspace rate and bandwidth caps (defaults set from measurement in t006, not chosen here), and has a kill switch: scale objectgw to zero replicas in GitOps. That needs no new mechanism and no environment variable. A stopped `pub-` host then gets Traefik's no-backend answer, which I expect to be 503 but did not verify, instead of objectgw's 404; an exact 404 would need a runtime flag store, which the backend does not have today (a search of `lego/backend/internal` for one found none) and which is a separate decision. The account is shared with the platform's backups (D1), so a Wasabi limit would reach the restore paths too; t006 adds an operator alert on month-to-date public egress, its threshold set from measurement like the caps, ahead of the kill switch. Cloudflare labels r2.dev non-production; bex's host is rate-limited and cached but not a CDN, and the docs must say so.
- **Not here:** custom domains, a CDN, signed public URLs.

### D5 — Lifecycle rules run in the sweep

`lifecycle set` stores `{rules}` on the bucket row. The hourly sweep (D8) deletes objects matching a rule's prefix whose `LastModified` is older than `maxAge` (or past `date`), at most 10,000 per rule per sweep, so one sweep stays inside its hour. Rules with `storageClassTransitions` answer `10042`; `abortMultipartUploadsTransition` is stored and does nothing, since v1 has no multipart. The cap is 1000 rules per bucket (bex's own). Wasabi's 90-day minimum storage duration bills an object deleted earlier; whether it bills the remainder for a deleted object is unverified, and the docs must warn that a short rule saves nothing on storage.

### D6 — Authorization is workspace-level, in two layers, for both contracts

Render has no bucket-level access control that I could find: access is the workspace role plus the API key, and a key today carries every permission of the account ([fine-grained keys](https://render.com/changelog/scope-access-to-a-workspace-s-resources-with-api-keys) are a private beta). bex mirrors Render's roles ([ADR024](ADR024-members.md), `deploy/gitops/authz/model.fga`), so access is by workspace and needs no new FGA type.

- **OpenFGA**, through `Base.AuthorizeWorkspace` (`lego/backend/internal/core/base.go:1231`) on `ownerId` or `accountId`:

  | Operation | Relation |
  | --- | --- |
  | list objects (A), list and info of buckets, `cors list`, `lifecycle list`, `dev-url get`, `domain list` | `can_view` |
  | get an object (A and B) | `can_view_sensitive` |
  | put or delete an object, bucket create or delete, `cors set\|delete`, `lifecycle set` | `can_create` |
  | `dev-url enable\|disable` | `can_manage` |

  Object bytes are customer data, as Postgres exports are, so get is sensitive, matching [ADR095](ADR095-agent-attachments.md).

- **OAuth scope class**, for a human OAuth token only: `bex.read`, `bex.write` or `bex.sensitive` (`lego/backend/internal/core/scope.go:217-232`). Machine API keys carry no scope and skip this layer ([ADR012](ADR012-auth.md)); they hold the `developer` role, so a key reaches every object of its workspace, as a Render key does. **The matrix row for Contract A's `GET …/{key...}` changes from `OpClassRead` to `OpClassSensitive`** (`scope_matrix_ops.go:800`, beside the Postgres export at `:810`); left as read, a developer's token that consented to `bex.read` only could mint a presigned GET for the bytes. Contract B's object get is `sensitive`, its public toggle `write`, and the rest follow the table. t002 adds a test for each.
- **Out of scope:** per-bucket or per-prefix credentials. Cloudflare scopes tokens to buckets, not prefixes; that belongs to the S3 API follow-up (D10).

### D7 — Names and keys are validated by bex and default closed

- **Bucket names:** wrangler's rule (D1), plus the reserved `render-` prefix.
- **Keys:** reject an empty key, one over 1024 bytes, one that starts with `/`, one with a `.` or `..` segment, and one with a backslash or a control character, on both contracts.
  - Reason: "Wasabi's handling of `..` in S3 keys" is unproven on a live deployment ([m394](../.pm/w21/done/m394/README.md)). A `..` segment is the one input that could step out of `<tea-id>/<obk-id>/`.
  - `http.ServeMux` cleans and redirects such paths, but the check runs on the decoded value regardless, with cases for `%2e%2e`, doubled slashes and Unicode.
  - Neither Render's nor Cloudflare's key rules are fully known (Cloudflare's `10020` InvalidObjectName has no published definition), so this is recorded in ADR018 as a bex rule.

### D8 — Usage, quotas and billing: the bucket is the truth, the gate is at write, v1 is a shadow meter

- **Gate.** A put is refused (`429` on A, `403` `10042` on B) when the last observed usage plus its size would pass a cap. On A the size is `sizeBytes` and the bytes bypass bex, so a per-workspace mint rate limit bounds the overshoot. On B bex sees the length first and the bytes after, so the overshoot is the concurrent streams times 300 MiB.
- **Sweep.** Hourly, per workspace and region, list `<tea-id>/`, attribute bytes and objects to a bucket by the second path segment, and write `object_storage_usage(workspace, bucket, region, bytes, objects, observed_at)`. The same pass runs the lifecycle rules (D5) and retries purges (D9). It feeds the shadow meter `object_storage_gb_seconds`, following [ADR023 § Sandbox snapshot storage](ADR023-usage-metering.md#sandbox-snapshot-storage-w22m313-adr104-d8) and [ADR104 D8](ADR104-sandbox-render-parity.md)'s precedent.
- **Transfer meters.** Contract B and objectgw see every byte, so `object_storage_ingress_bytes` and `object_storage_egress_bytes` are exact for them and an estimate for Contract A (D2). Shadow only.
- **No Stripe Price yet.** Render has published no price ([ADR030](ADR030-pricing.md)'s convention is Render's price × 0.70, which needs one), and the Cloudflare-shaped layer has no Render price to follow at all. v1 records the quantities and shows them at $0, like `sandbox_memory_gb_seconds`. For the user's later decision, the numbers on the table are Wasabi's floor of about $0.008 per GB-month and Cloudflare's published R2 list price, read 2026-11-14: $0.015 per GB-month, $4.50 per million Class A and $0.36 per million Class B requests, egress free. bex's other storage rates in `pricing.yaml` are $0.175 disk, $0.21 Postgres and $0.056 snapshot.
- **One cost-denominated Stripe dimension, not one per meter.** A Stripe Subscription holds at most 20 items ([ADR040 §1](ADR040-billing-metronome.md)). The catalog had 18 dimensions on 2026-11-05 and has gained the snapshot SKU since; I did not count the current total. Storage, transfer and requests would share one `object_storage_cost_microusd` at $0.000001 per unit, rated by bex-api from `pricing.yaml`, as `key_value_cost_microusd` is. Hobby follows `pricing.yaml` and stays free.
- **Caps (bex's own, constants in `internal/objectstorage`, not environment variables, D9):** 100 GB and 1,000,000 objects per workspace per region, 1,000 buckets per workspace, 5 GiB per object on A (the S3 single-PUT ceiling; Wasabi's own limit is unverified) and 300 MiB on B, 8 concurrent streams per workspace. The object cap exists so one sweep finishes inside an hour. Each is recorded in ADR018 as a dated, unverified entry, and changing one is a reviewed code change with a new dated entry. Tests inject smaller values through the config struct.

### D9 — Credentials and rollout reuse the ADR095 and ADR029 recipes

- **Identities.** The platform's existing Wasabi account (D1) gets two IAM users of its own, the split [ADR029](ADR029-static-sites.md) uses: `infra/wasabi/object-storage-s3-policy.json` (get, put, delete and list on `bex-objects-oregon` and `bex-objects-frankfurt`, held by bex-api) and `infra/wasabi/object-storage-read-s3-policy.json` (get and list on the same two, held by objectgw). The policies name the buckets literally, as `agent-attachments-s3-policy.json` does, so a new region is a policy change. Neither identity may touch `bex-tfstate` or any other bucket, and `verify` proves it with a denied probe on each. That matters more on a shared account than it would on a separate one.
- **Script and root credential.** `scripts/object-storage-s3-credentials.sh {provision,verify,install}`, idempotent, never printing a secret. No new root variable: it uses the out-of-band `TF_STATE_ACCESS_KEY` and `TF_STATE_SECRET_KEY` from `.env`, or `AWS_ACCESS_KEY_ID` and `AWS_SECRET_ACCESS_KEY` when those are set, which is also how a later separate account would be reached (`scripts/agent-attachments-s3-credentials.sh:50-56`). `provision` creates the two buckets and the two users and writes the two key pairs below to `.env`; `install` creates the Secrets `bex-system/bex-object-storage` and `bex-system/bex-object-storage-read`.
- **Variables: a credential and nothing else.** An environment variable here carries a secret. Endpoints, bucket names, regions, the default region, the caps (D8) and wrangler's 300 MiB limit are constants in `internal/objectstorage`, so changing one is a reviewed code change. The endpoint is `https://s3.<wasabi-region>.wasabisys.com`, the bucket is `bex-objects-<region>`, and the Wasabi region comes from D1's map (`oregon` to `us-west-1`, `frankfurt` to `eu-central-2`). This departs from [ADR095](ADR095-agent-attachments.md)'s `BEX_AGENT_ATTACHMENTS_S3_{ENDPOINT,BUCKET,REGION}`, which stay as they are. Four variables, mirrored in `.env.example` and the `lego/backend/AGENTS.md` inventory, with the [ADR019](ADR019-infra-credentials.md) rows added:
  - bex-api, from `bex-system/bex-object-storage`: `BEX_OBJECT_STORAGE_ACCESS_KEY` and `BEX_OBJECT_STORAGE_SECRET_KEY`. An IAM user belongs to the account, so one pair serves both regions.
  - objectgw, from `bex-system/bex-object-storage-read`: `BEX_OBJECT_STORAGE_READ_ACCESS_KEY` and `BEX_OBJECT_STORAGE_READ_SECRET_KEY`, one read-only pair for both regions. objectgw never receives the read-write pair.
  - Reused, already set: `BEX_BASE_DOMAIN`.
  - Tests reach a fake S3 by injecting the endpoint through the config struct. A local S3 for development would be its own decision, not an environment override.
- Without the credential pair, both regions keep answering today's `404` "not offered" on Contract A and `10042` on Contract B. The code ships before any bucket exists and nothing changes until the user installs the credential, the shape of [ADR095](ADR095-agent-attachments.md)'s 503-when-unconfigured path.
- **Deleting a workspace or account purges `<tea-id>/`** in both regions and drops its bucket rows, so no `pub-` host keeps answering ([ADR086](ADR086-account-deletion.md)). An unavailable bucket is retried by the sweep, as ADR095 D7 does for attachments. Without it, a deleted workspace's objects keep accruing Wasabi's 90-day minimum for nothing.

### D10 — Not in this decision

Each is its own follow-up, per the rule that parity-sufficient work ships and wider variants are filed separately:

- **An S3-compatible API with tenant credentials.** It is what makes aws-cli, boto and rclone work, and what lists a named bucket, since wrangler cannot. The open choice is bex verifying SigV4 over virtual buckets, against per-tenant Wasabi IAM users scoped to a prefix (whether Wasabi enforces prefix-scoped policies is unverified). It reverses the first version's "no S3 credentials to tenants", which was written for Render's contract.
- Custom domains for public buckets, multipart upload, versioning, object lock, event notifications, Sippy migration, Data Catalog, storage classes.
- A dashboard UI, GraphQL and MCP adapters (D1's core service makes them thin), and anything specific to the Render SDK clients beyond D11's test.

### D11 — Both contracts are pinned by tests, and the docs say how to use them

- **Contract A.** Delete the launcher refusal and `lego/cli/object_storage_test.go` (`--local` was never intercepted). Add a contract test that drives the pinned `ea objects put|get|list|delete` against a fake S3 (`httptest`), in the style of `sandbox_contract_test.go`: the zero-byte upload, a 413, a 404 on get and on delete, `--limit=200`. Flip the four checklist rows from `[-]` to `[x]`. If the Python and TypeScript clients accept a custom base URL (unverified), run them against the same fake; otherwise pin their routes and the `maxSizeBytes == sizeBytes` rule with a wire-shape test written from their source.
- **Contract B.** A Go test replays the request fixtures recorded from the real wrangler run (method, path, headers, body) against the handlers with a fake S3, and checks every answer parses as the envelope. `scripts/wrangler-r2-contract.sh` installs the pinned **wrangler 4.162.0** into a temporary directory, starts a bex-api over the fake S3, and runs the commands of the table. It runs weekly with the other drift jobs and on any wrangler pin change, not on every PR, because wrangler is Cloudflare's tool and its wire can change in any release.
- **Docs.** `docs/bex-cli.md` gets a section: the three variables, `--remote`, that listing needs the Render CLI for `render-<region>` until the S3 API exists, and the public-bucket risk and egress notes.
- **Isolation tests:** `..`, `%2e%2e`, doubled slashes, 1024 bytes, Unicode, another workspace's `obk-` id, a presigned URL signed for another key, and a bucket name reused after delete.

## Alternatives considered

- **Keep option B (not offered).** It contradicts the parity rule and the user's instruction.
- **Serve only Render's four routes (this ADR's first version).** It has no bucket, no public access, no CORS and no lifecycle; Render itself calls the feature early access.
- **Serve only wrangler's contract.** It loses `render ea objects`, the SDKs and the ADR018 row that the parity rule requires, for the price of one reserved bucket name.
- **Cloudflare R2 as the backend.** Its native public domains, CORS, lifecycle, bucket-scoped tokens and free egress would make D4, D5 and the S3 follow-up almost free, and it allows 100,000 buckets per account. The user chose a Wasabi wrapper. R2's costs are location hints that are only best effort (only jurisdictions guarantee placement), at most one write per second to the same key, a second vendor with no infrastructure in this repository, and storage at nearly twice Wasabi's floor. **Reassess it if Wasabi's fair-use limit stops public buckets.**
- **A separate Wasabi account for tenant data (this ADR's first draft).** It keeps a Wasabi egress action away from the backups and restore paths, for a second 1 TB minimum (about $8 a month) and a second root credential to guard. The user chose the shared account on 2026-11-15. Revisit it at the first Wasabi warning or at the egress alert (D1, D4).
- **The Cloudflare REST API as bex's own native contract, with an S3 API for bytes.** It is D3 without the streaming, and it is not what wrangler does: wrangler's object commands carry the bytes in the REST call.
- **A physical Wasabi bucket per customer bucket.** It hits the 1000-bucket default and isolates nothing a key prefix does not.
- **Redirect wrangler's PUT to a presigned URL.** See D3; it probably cannot work with a streamed body.
- **Extend the static-server for public buckets.** It learns hosts from App custom resources and has no database access by design; a bucket has no CR. objectgw copies its shape instead.
- **MinIO or Hetzner Object Storage.** MinIO adds a PVC and an operations surface ([ADR082](ADR082-persistent-disks.md)). `s3client.go` notes Hetzner works too, but the user chose Wasabi and its account, IAM scripts and presign path are in place.
- **A Stripe item each for storage, transfer and requests.** The 20-item Subscription cap leaves no room (D8).

## Consequences

- **bex-api becomes a data plane for Contract B.** Streamed uploads and downloads cost connections, bandwidth and memory-free but long-lived goroutines on the control plane. The 300 MiB cap, the concurrency caps and the deadline extension bound it; they do not remove it. If it hurts, move Contract B's bytes into objectgw.
- **A new external cost surface.** Wasabi's 1 TB minimum, 90-day minimum storage duration and egress fair-use all respond to tenant behavior, and bex's invoice has no meter to push them back. Public buckets sharpen the egress risk, and the account is shared with the backups, so a limit would reach the restore paths too (D1). The shadow meters, the caps and the kill switch keep it bounded until pricing exists.
- **Provider concentration.** Tenant data inherits the single-provider, single-region exposure `.pm/FUTURE-MAYBE.md:23` records for backups. v1 has no replica, and the docs must say so.
- **Two third-party wire shapes to chase.** Render's `ea` namespace is "subject to change" and wrangler releases often. The contract tests are the only early warning.
- **The gap in listing.** wrangler cannot list objects and Contract A lists only the system bucket, so a customer cannot list a named bucket from any CLI until the S3 API exists (D10). I consider this the main usability hole in this decision.
- **A presigned URL is a bearer credential** for one key until it expires, and a public bucket's objects are readable by anyone.
- **Using Cloudflare's names.** The path says `cloudflare`. As with Render, bex may refer to Cloudflare to explain compatibility and may not claim to be it; no answer says so.
- **Quota is eventually consistent.** Gating at write plus rate limits bounds the overshoot but does not remove it.
- **A new Stripe dimension is scarce.** One cost-denominated dimension (D8) is all the catalog can afford.

## Tasks (filed as three milestones)

`/pmx adr 112 to w21` (2026-11-15) filed this split on the board as three milestones, each a shippable step, with the tasks below cut finer (tens of minutes each):

- [w21/m415](../.pm/w21/blocked/m415/README.md): the core store and Contract A (t001, t002, t004 and the Contract A parts of t008 and t009), ending in a production walk.
- [w21/m416](../.pm/w21/m416/README.md): Contract B, the sweep, lifecycle, purge and the shadow meters (t003, t005, t007 and the Contract B parts of t008 and t009), ending in a production walk.
- [w21/m417](../.pm/w21/m417/README.md): public reads, objectgw and its caps, alert and kill switch (t006 and the public part of t009).

t010 (pricing) is not filed: it waits for the user to set a price.

| # | Task |
| --- | --- |
| t001 | Migration for `object_buckets`, the `obk` id kind, and `internal/objectstorage` (bucket registry, key layout, validation, quota gate, Wasabi client) with a fake S3 |
| t002 | Contract A routes replacing the not-offered refusals; the system bucket; the scope matrix row for get becomes `OpClassSensitive`, with a test that a `bex.read`-only developer token cannot mint a GET URL; the pinned-CLI contract test; launcher pass-through; the checklist rows |
| t003 | Contract B routes under `/cloudflare/client/v4` for buckets and objects, the envelope and error codes, deadline extension and stream caps, the `getR2StorageMetrics` answer, `classifiedOps` entries, the recorded-fixture test and `scripts/wrangler-r2-contract.sh` |
| t004 | `infra/wasabi` policies, `scripts/object-storage-s3-credentials.sh` (its `verify` includes the denied probe on `bex-tfstate`), the ADR019 rows, `.env.example`, the `lego/backend/AGENTS.md` inventory |
| t005 | Hourly sweep, `object_storage_usage`, lifecycle execution, the shadow storage and transfer meters, the `cors` and `lifecycle` routes |
| t006 | objectgw, its Deployment and read credential, the `dev-url` routes, the Traefik route for `pub-*` hosts, the `pub-` slug reservation, rate and bandwidth caps, an operator alert on month-to-date public egress, the kill switch (scale to zero, D4), and tests for private, unknown, `Range`, CORS and listing |
| t007 | Workspace and account delete purge of the prefix, the bucket rows and the gateway cache, with the retry sweep |
| t008 | Docs: the `bex-cli.md` section, the ADR018 row and its dated divergences, a catalog line; `npx prettier@3.4.2 --write "**/*.md"` |
| t009 | Production walk (needs the user to run `provision` and `install` of `scripts/object-storage-s3-credentials.sh` with the existing root credential): 0 B, 1 B and 5 MiB round trips byte-identical on both CLIs, a 300 MiB wrangler upload through Traefik, 5 GiB + 1 refused, a cross-workspace key refused, `Content-Length` enforcement probed, a streamed presigned PUT probed, a public GET with `Range` and CORS, the kill switch |
| t010 | Deferred until the user sets a price: the `pricing.yaml` rate, the `object_storage_cost_microusd` dimension in `scripts/stripe-billing-setup.py` after counting the catalog's items, then switching the meter on (D8) |

## Not verified

- **Render:** the EA region set, price, quotas, presign lifetime, the list default and cap, key rules, delete-missing status, overwrite semantics, whether roles or fine-grained keys treat object operations as a permission of their own, and whether buckets will ever be added. The evidence is the pinned CLI and SDK source and a few feedback-board posts.
- **The SDKs:** whether they can be pointed at a non-Render base URL (D11).
- **wrangler:** only the commands listed under Contract B as run. The rest come from source. Any other wrangler version may differ; 4.162.0 is the one measured. I did not check what wrangler does with a real GraphQL answer for `bucket info`, or its behavior for a bodiless 2xx beyond reading the source.
- **Cloudflare:** the meaning of `10020` and of any code not in the published table; R2's overwrite and ETag semantics, which bex does not copy exactly.
- **Wasabi:**
  - **Settled 2026-11-15 on production Wasabi (`us-west-1`, w21/m415 t010):** a presigned PUT whose signature covers `Content-Length` is enforced. A body of exactly the signed length is stored (200); one longer, one shorter, and one sent chunked with no length are all refused with 403 `SignatureDoesNotMatch`. So Contract A's size cap rests on Wasabi, not on the mint rate and the sweep, and a streamed PUT (Contract B) must send a real `Content-Length`, never chunked encoding. Not yet probed: a body streamed from a network reader, which is the same bytes on the wire as a fixed-length one (m416 t003's test and walk confirm it).
  - **Settled 2026-11-15 (same probe):** a key with a `..` segment, written through a presigned URL, is stored literally as `verify/x/../…`, not normalized. D7's refusal stays, as defence in depth and because other S3 clients and URL cleaners do walk paths.
  - Whether the 90-day minimum bills deleted objects for the remainder.
  - The single-PUT size limit, and whether `LastModified` suits lifecycle `maxAge`.
  - Whether prefix-scoped IAM policies are enforced (D10).
  - Whether Wasabi acts on a whole account rather than on one bucket when egress exceeds storage, and how it measures the ratio. That decides what the shared account risks (D1).
  - Which Wasabi regions correspond to `ohio`, `virginia` and `singapore`.
- **bex:** how Traefik should route `pub-*` hosts to objectgw (an IngressRoute `HostRegexp` or operator-managed Ingresses); where service slugs are validated for the `pub-` reservation; what Traefik answers for a `pub-` host while objectgw has no replicas (D4); whether a 300 MiB upload crosses Traefik and bex-api cleanly (Traefik's `readTimeout` is 6000 s, the 30 s server timeouts are overridden per request); what the nodes' egress budget allows for public traffic.
- **Stripe:** the catalog's current number of dimensions (D8). I counted 18 from ADR040 and did not run the setup script.
- **Decisions the user can still change without blocking the code:** sharing the platform's Wasabi account rather than using a separate one (D1, decided 2026-11-15), the `render-<region>` system bucket that joins the two contracts (D1), the public host on `onbex.co` (D4), the default caps (D8), the default region for a bucket created without a hint (D1), and "free until a price is set" (D8).

## Sources

- [Cloudflare wrangler `r2` commands](https://developers.cloudflare.com/r2/reference/wrangler-commands/) and its [source](https://github.com/cloudflare/workers-sdk/tree/main/packages/wrangler/src/r2)
- [R2 public buckets](https://developers.cloudflare.com/r2/buckets/public-buckets/), [limits](https://developers.cloudflare.com/r2/platform/limits/), [pricing](https://developers.cloudflare.com/r2/pricing/), [data location](https://developers.cloudflare.com/r2/reference/data-location/), [API tokens](https://developers.cloudflare.com/r2/api/tokens/), [S3 API compatibility](https://developers.cloudflare.com/r2/api/s3/api/), [presigned URLs](https://developers.cloudflare.com/r2/api/s3/presigned-urls/) and [error codes](https://developers.cloudflare.com/r2/api/error-codes/)
- [Render Cloud Object Storage feedback board](https://feedback.render.com/features/p/cloud-object-storage)
- [Render Object Storage](https://render.com/object-storage)
- [Render workspaces, members and roles](https://render.com/docs/team-members)
- [Render fine-grained API keys, private beta (changelog, 2025-10-27)](https://render.com/changelog/scope-access-to-a-workspace-s-resources-with-api-keys)
- [render-oss/sdk](https://github.com/render-oss/sdk), its [Python object client](https://raw.githubusercontent.com/render-oss/sdk/main/python/render/experimental/object/client.py) and [TypeScript object client](https://raw.githubusercontent.com/render-oss/sdk/main/typescript/src/experimental/object/client.ts)
- [render on PyPI, 1.8.0](https://pypi.org/project/render/1.8.0/) and [render-oss/cli releases](https://github.com/render-oss/cli/releases)
- `render-oss/cli@v1.1.3-0.20261106232710-4fd34231b637`, `pkg/storage` and `cmd/object*.go`
- [Wasabi pricing](https://wasabi.com/pricing), [pricing FAQ](https://wasabi.com/pricing/faq) and [service URLs](https://docs.wasabi.com/docs/what-are-the-service-urls-for-wasabi-s-different-storage-regions)
