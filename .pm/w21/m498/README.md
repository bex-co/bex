# w21 · m498 — S3 core: SigV4, access keys, buckets, objects, listing and multipart on a `bex-s3` Deployment

**Worker:** worker21 **Goal:** any S3 client (aws-cli, rclone, boto3) with a bex access key and `https://objects.onbex.co` as its endpoint lists, uploads (single and in parts), copies, downloads and deletes objects in the workspace's buckets, with quota, metering, audit and limits in bex's path. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](t001.md) | The Wasabi policy gains the multipart actions, and `verify` proves them | 30m | — |
| [t002](t002.md) | Access keys: tables, the `s3k` id kind, the core service and the OpenBao secret | 60m | — |
| [t003](t003.md) | Key disposal, the 2FA and sign-in gates, the REST and GraphQL adapters, the bucket route and the publish route | 60m | t002 |
| [t004](t004.md) | `bex-s3`: the entry point and the Deployment | 60m | t002 |
| [t005](t005.md) | The SigV4 verifier and the router | 60m | t002, t004 |
| [t006](t006.md) | Payload modes: `aws-chunked`, the length rule and integrity before the commit | 60m | t005 |
| [t007](t007.md) | The `objects.` IngressRoute, the validation pins and the failed-authentication limit | 45m | t004, t005 |
| [t008](t008.md) | Bucket operations and configuration | 60m | t005 |
| [t009](t009.md) | Object operations: put, get, head, delete, delete-many, copy and tagging | 60m | t006, t008 |
| [t010](t010.md) | The quota gate and the limits: write-through counter, pacing, rates and the egress budget | 60m | t009 |
| [t011](t011.md) | Audit rows for S3 | 40m | t009 |
| [t012](t012.md) | Listing: `ListObjectsV2` and `ListObjects` | 45m | t008 |
| [t013](t013.md) | Multipart upload, `UploadPartCopy` and the sweep's cleanup | 75m | t009, t010 |
| [t014](t014.md) | Observability, the kill switch and the S3 runbook | 50m | t007, t010 |
| [t015](t015.md) | The Go suite on the AWS SDK for Go v2 and the isolation tests | 60m | t012, t013 |
| [t016](t016.md) | Render parity | 30m | t015 |
| [t017](t017.md) | Simplify | 20m | t016 |
| [t018](t018.md) | Test coverage | 45m | t016, t017 |
| [t019](t019.md) | Closeout | 10m | t018 |

## Definition of done

1. A key minted for a workspace member signs requests that `bex-s3` verifies; a wrong signature, an expired, revoked or unknown key, a skewed clock, an unsigned `x-amz-*` header and a key whose creator lost the role are all refused with S3's XML error.
2. `CreateBucket`, `ListBuckets`, `HeadBucket`, `DeleteBucket`, `GetBucketLocation`, CORS and lifecycle work through stock aws-cli; the bucket-policy calls answer 501.
3. `PutObject` (a body with `Content-Length` and an `aws-chunked` trailer body without one), `GetObject` (with `Range`, 416 past the end), `HeadObject`, `DeleteObject`, `DeleteObjects`, `CopyObject`, `UploadPartCopy` and multipart upload round trip byte-identical; a checksum mismatch stores nothing and leaves a previous object untouched.
4. `ListObjectsV2` and `ListObjects` page correctly with and without a delimiter (a page that ends on a common prefix does not repeat it).
5. The quota gate counts completed writes and unfinished multipart bytes; reads are paced and capped by the egress budget; the audit rows, limits and metrics of ADR112 D3, D6 and D8 exist and the kill-switch step is documented.
6. `bex-s3` runs as its own Deployment with a grace period above the stream deadline and serves no REST, GraphQL, MCP or OAuth route; the new `objects.` IngressRoute sends the rest of the host to it while Contract B's route is untouched.

## Source + Goal linkage

- **Source:** [ADR112](../../../docs/ADR112-object-storage.md) (amended 2026-11-19: Contract B replaced by an S3-compatible API and `bex s3`), filed by `/pmx adr 112 to w21` on 2026-11-19; the S3 amendment's tasks t101 to t114 (§ Tasks), reorganized into three milestones.
- **Goal linkage:** ADR008 Render parity first (the S3 API is an `Extension`, not a parity break, and Contract A keeps its contract) and the user's 2026-11-14 goal of serving customers' object needs, restated on 2026-11-19 as S3 compatibility.
- **Expected outcome:** A customer points any S3 client at `objects.onbex.co` with a bex access key and manages buckets and objects, including listing a named bucket, which no CLI could do before.
- **Why now:** The user chose S3 on 2026-11-19 because `wrangler r2` cannot list, copy or sync, and Contract B shipped on 2026-11-15; this is the server half of the replacement. It ships first because `bex s3` (m499), the production walk and the removal of Contract B (m500) all need a live S3 surface, and the new route must exist before Contract B's route can go.
- **Decision state.** The user decided on 2026-11-19: serve the S3 protocol instead of `wrangler r2`'s contract, bex verifies SigV4 itself and streams the bytes ("option A"), the command is `bex s3 <…>`, and the R2 CLI approach is removed entirely. **Recommendations of the ADR, recorded here as recommendations and not as user decisions** (selecting this milestone schedules them; ADR112 § Not verified lists what is unverified): `bex-s3` as a Deployment of its own; no access key holds `can_manage`, so publishing is `bex s3 public`, not an S3 bucket policy; path-style addressing only; `bex s3` runs the user's `aws` rather than a native client; reads paced at 8 MiB/s per workspace per replica, uploads at 16 MiB/s, and a monthly S3 egress budget of the larger of the stored bytes and 5 GiB; 12-hour ephemeral keys; integrity verified before the commit by withholding the last byte; the second IngressRoute before Contract B is removed. Each is a dated, unverified ADR018 entry when it ships.
