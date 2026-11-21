# w21 · m540 — `bex s3` speaks S3 itself: a native client with no other tool to install, then `bex-cli` v0.9.0

**Worker:** worker21 **Goal:** `bex s3 ls`, `cp`, `mv`, `rm`, `sync`, `mb`, `rb`, `presign`, `cors`, `lifecycle` and `info` work on a machine with nothing else installed, no code path runs another program and no document says it does, and the release `bex-cli` v0.9.0 carries it. **Status:** todo (t001 to t008 done)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | The client core: AWS SDK for Go v2 in `lego/cli`, a client built from the cached key, error mapping | 1h30m | — **DONE** |
| [t002](done/t002.md) | `ls`, `mb`, `rb`, `rm` and `presign` | 1h15m | t001 — **DONE** |
| [t003](done/t003.md) | `cp` and `mv`: upload, download and bucket-to-bucket copy | 2h | t001 — **DONE** |
| [t004](done/t004.md) | `sync` between a directory and a prefix and between two prefixes | 2h | t003 — **DONE** |
| [t005](done/t005.md) | `cors`, `lifecycle` and `info` for an object | 1h | t001 — **DONE** |
| [t006](done/t006.md) | Remove the launcher: `s3api`, the process replacement and every path that runs another program | 45m | t002, t003, t004, t005 — **DONE** |
| [t007](done/t007.md) | Tests: a fake-S3 suite for each verb, and `bex s3` as one more client of `scripts/s3-contract.sh` | 1h30m | t006 — **DONE** |
| [t008](done/t008.md) | Docs: no installation requirement, ADR018, the README, the runbooks, and every reference to another tool deleted | 1h15m | t007 — **DONE** |
| [t009](t009.md) | Production walk with a tree-built `bex` on a machine with no other S3 tool | 1h30m | t008 |
| [t010](t010.md) | Render parity | 30m | t009 |
| [t011](t011.md) | Simplify | 20m | t010 |
| [t012](t012.md) | Test coverage | 45m | t010, t011 |
| [t013](t013.md) | Release `bex-cli` v0.9.0 and verify every channel | 45m | t012 |
| [t014](t014.md) | Closeout | 10m | t013 |

## Definition of done

1. On production, from a machine with no other S3 tool on its PATH, a tree-built `bex` runs `ls`, `mb`, `cp` (0 B, 1 B, 20 MiB and 1 GiB round trips by sha256), `sync` (a second run copies nothing), `mv`, `rm --recursive`, `presign`, `cors`, `lifecycle` and `info`, and ten concurrent commands on an empty key cache make one key; the walk is recorded in ADR112 § Not verified.
2. No code path of `bex s3` runs another program: `launch.go`, `s3api`, the bex-owned config files and the missing-tool error are gone, and `git grep -n -i -w -e aws -e s3api -- lego/cli docs/bex-cli.md docs/ADR018-render-parity.md lego/cli/README.md` finds no line that says `bex s3` runs, wraps or mirrors another tool (the SDK's import lines and module names are expected).
3. `scripts/s3-contract.sh` runs the tree-built `bex` beside aws-cli, rclone and boto3 and is green; `make lint` and the telemetry coverage contract pass.
4. `docs/bex-cli.md`, ADR018, `lego/cli/README.md`, the runbooks and the catalog describe the native command; the two `BEX_API_KEY` sentences say "since v0.8.0".
5. `bex-cli/v0.9.0` is published, signed and verified on every channel, and `bex s3 ls` ran from the installer's binary on a machine with no other S3 tool.

## Source + Goal linkage

- **Source:** [ADR112](../../../docs/ADR112-object-storage.md) (amended 2026-11-20: `bex s3` is a native client; § Tasks, t115 to t124 are this milestone's t001 to t009 and t013, and its t010 to t012 and t014 are the standing closing tasks), filed by `/pmx adr 112 to w21` on 2026-11-20 after the user ran `bex s3 ls` from `bex-cli` v0.8.0 and got an error that the AWS CLI was not on the PATH.
- **Goal linkage:** The user's 2026-11-19 decision (S3 with `bex s3 <…>`) and their 2026-11-20 correction (an S3-compatible bex CLI that needs nothing else; every reference to the other tool deleted), the `bex-cli/v*` release train, ADR113's coverage contract for extended commands.
- **Expected outcome:** A customer runs `bex s3 sync ./site s3://photos/site` after `bex login`, on a machine with no other S3 tool, and `bex s3 cors set photos --file cors.json` configures a bucket.
- **Why now:** `bex-cli` v0.8.0 shipped `bex s3` as a launcher over a separately installed tool, and the first customer run failed on the missing tool. The server is done and walked on production (m498 to m500); only the client layer is wrong.
- **Decision state.** The user decided on 2026-11-20: `bex s3 <…>` is an S3-compatible bex CLI and not a wrapper, `bex s3api` goes, and no reference to the other tool remains. **Recommendations of the plan, recorded here as recommendations and not as user decisions** (selecting this milestone schedules them): the AWS SDK for Go v2 with the GA `feature/s3/manager` and not the v0.x `transfermanager`; verbs and output lines in the familiar S3-tool shape with bex's own flags; 8 requests in flight by default; 8 MiB parts grown to stay under 10,000; `info` also inspects an object; `cors` and `lifecycle` read the S3 API's JSON rule documents; a name bex does not define is `unknown command`; `bex s3api` is removed from v0.8.0's surface in v0.9.0; the release is `v0.9.0` (a minor, by Conventional Commits). The sign-offs still open: ADR018's dated divergences for the new surface, ADR112's status ("proposed"), and ADR112 t010 (pricing).
