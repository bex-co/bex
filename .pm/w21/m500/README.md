# w21 · m500 — S3 walked on production, then Contract B removed

**Worker:** worker21 **Goal:** the S3 surface is proven on production with the stock clients, and the Cloudflare-shaped routes, the wrangler fixtures, the weekly wrangler job and their docs are gone. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | The production walk of the S3 surface | 90m | w21/m498/t019, w21/m499/t009 — **DONE** |
| [t002](t002.md) | The removal gate and the removal of Contract B's routes and code | 60m | t001 |
| [t003](t003.md) | Remove the wrangler CI, scripts and docs, and finish the dead-code pass | 40m | t002 |
| [t004](t004.md) | ADR112: record the walk and the removal | 30m | t003 |
| [t005](t005.md) | Render parity | 30m | t004 |
| [t006](t006.md) | Simplify | 20m | t005 |
| [t007](t007.md) | Test coverage | 45m | t005, t006 |
| [t008](t008.md) | Closeout | 10m | t007 |

## Definition of done

1. The production walk (ADR112 t109) passed: 0 B, 1 B and 20 MiB multipart round trips byte-identical with aws-cli, rclone and boto3; a 1 GiB multipart upload; `aws s3 sync` of a tree; `aws s3 mv` of a 20 MiB object; a cross-workspace key refused; a revoked key refused; a presigned URL past its expiry refused; the `Content-Length` and trailer handling on production Wasabi; a rollout during an upload; the probes of ADR112 § Not verified answered and written there.
2. Contract B received zero requests in the seven days before removal and no non-system bucket exists outside the QA workspaces; then the routes, tests, fixtures, scripts, workflows, scope-matrix entries, dead code and docs listed in ADR112 D3's removal bullet are gone and `make lint` and the backend suite pass.
3. ADR112's status and Not verified section record the walk's results and the removal.

## Source + Goal linkage

- **Source:** [ADR112](../../../docs/ADR112-object-storage.md) (amended 2026-11-19: Contract B replaced by an S3-compatible API and `bex s3`), filed by `/pmx adr 112 to w21` on 2026-11-19; the S3 amendment's tasks t101 to t114 (§ Tasks), reorganized into three milestones.
- **Goal linkage:** The user's 2026-11-19 decision to remove the R2 CLI approach entirely, and the repo rule that a divergence or surface stands only while something records it.
- **Expected outcome:** `objects.onbex.co` serves only S3; no code, CI job or document describes wrangler as a way to use bex.
- **Why now:** Contract B shipped on 2026-11-15 and the user rejected it on 2026-11-19; it stays until the S3 surface has been walked on production, so there is never a time without a way to list or manage a bucket. Removing it ends a second internet-facing surface that nobody should have to secure or test.
- **Decision state.** The user decided on 2026-11-19: serve the S3 protocol instead of `wrangler r2`'s contract, bex verifies SigV4 itself and streams the bytes ("option A"), the command is `bex s3 <…>`, and the R2 CLI approach is removed entirely. **Recommendations of the ADR, recorded here as recommendations and not as user decisions** (selecting this milestone schedules them; ADR112 § Not verified lists what is unverified): `bex-s3` as a Deployment of its own; no access key holds `can_manage`, so publishing is `bex s3 public`, not an S3 bucket policy; path-style addressing only; `bex s3` runs the user's `aws` rather than a native client; reads paced at 8 MiB/s per workspace per replica, uploads at 16 MiB/s, and a monthly S3 egress budget of the larger of the stored bytes and 5 GiB; 12-hour ephemeral keys; integrity verified before the commit by withholding the last byte; the second IngressRoute before Contract B is removed. Each is a dated, unverified ADR018 entry when it ships.
