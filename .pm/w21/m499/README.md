# w21 · m499 — `bex s3`: the AWS CLI with bex's endpoint and keys, the S3 contract suite, and the docs

**Worker:** worker21 **Goal:** `bex s3 <…>` runs the AWS CLI against bex with no setup beyond `bex login`, stock aws-cli, rclone and boto3 are pinned by a weekly contract suite, and the docs and ADR018 describe the S3 surface instead of wrangler's. **Status:** todo (t001 to t008 done)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | The `bex s3` and `bex s3api` launcher — **DONE** | 60m | w21/m498/t003, w21/m498/t007 |
| [t002](done/t002.md) | `bex s3 env`, `keys`, `public` and `info` — **DONE** | 45m | t001 |
| [t003](done/t003.md) | CLI tests with a fake `aws` and the telemetry coverage contract — **DONE** | 45m | t002 |
| [t004](done/t004.md) | `scripts/s3-contract.sh`: pinned aws-cli v2, rclone and boto3, weekly — **DONE** | 60m | w21/m498/t015, t001 |
| [t005](done/t005.md) | Docs: `bex s3`, ADR018, the runbook and the inventories — **DONE** | 60m | t003, t004 |
| [t006](done/t006.md) | Render parity — **DONE** | 30m | t005 |
| [t007](done/t007.md) | Simplify — **DONE** | 20m | t006 |
| [t008](done/t008.md) | Test coverage — **DONE** | 45m | t006, t007 |
| [t009](t009.md) | Closeout | 10m | t008 |

## Definition of done

1. `bex s3 mb|cp|ls|mv|rm|sync|presign` and `bex s3api <call>` work against a bex with the m498 surface, using an ephemeral key minted for the signed-in person (twelve hours, replaced when under one hour remains); the user's own AWS profile and credentials are never sent to bex and bex's key never lands in `~/.aws`.
2. `bex s3 env`, `keys create|list|revoke`, `public <bucket> enable|disable|status` and `info <bucket>` work; a missing `aws` prints how to install it and points at `bex s3 env`.
3. `scripts/s3-contract.sh` runs pinned aws-cli v2, rclone and boto3 with their own defaults against a bex-api over a fake S3, weekly and on a pin change, and is registered in `scripts/ci-schedule-drift.sh`.
4. `docs/bex-cli.md` has a `bex s3` section in place of the wrangler one, ADR018 records the S3 `Extension` row and the dated divergences, and the runbook and inventories match.

## Source + Goal linkage

- **Source:** [ADR112](../../../docs/ADR112-object-storage.md) (amended 2026-11-19: Contract B replaced by an S3-compatible API and `bex s3`), filed by `/pmx adr 112 to w21` on 2026-11-19; the S3 amendment's tasks t101 to t114 (§ Tasks), reorganized into three milestones.
- **Goal linkage:** The user's 2026-11-19 decision (S3 with `bex s3`), the `bex` CLI launcher train (`bex-cli/v*`), and ADR113's coverage contract for extended commands.
- **Expected outcome:** A customer runs `bex s3 sync ./site s3://photos/site` after `bex login`, and `bex s3 public photos enable` publishes a bucket.
- **Why now:** The user's command is `bex s3 <…>`. It needs the m498 surface (keys, the endpoint) and is what makes S3 usable without hand-set environment variables; the contract suite is the only early warning when a client changes its defaults.
- **Decision state.** The user decided on 2026-11-19: serve the S3 protocol instead of `wrangler r2`'s contract, bex verifies SigV4 itself and streams the bytes ("option A"), the command is `bex s3 <…>`, and the R2 CLI approach is removed entirely. **Recommendations of the ADR, recorded here as recommendations and not as user decisions** (selecting this milestone schedules them; ADR112 § Not verified lists what is unverified): `bex-s3` as a Deployment of its own; no access key holds `can_manage`, so publishing is `bex s3 public`, not an S3 bucket policy; path-style addressing only; `bex s3` runs the user's `aws` rather than a native client; reads paced at 8 MiB/s per workspace per replica, uploads at 16 MiB/s, and a monthly S3 egress budget of the larger of the stored bytes and 5 GiB; 12-hour ephemeral keys; integrity verified before the commit by withholding the last byte; the second IngressRoute before Contract B is removed. Each is a dated, unverified ADR018 entry when it ships.
