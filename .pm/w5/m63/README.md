# w5 · m63 — Encrypted platform backups + write-scoped backup credentials (ADR050)

**Worker:** worker5 **Goal:** implement `docs/ADR050-encrypted-platform-backups.md` — client-side `age` encryption for the etcd/OpenBao/KeyValue backup pipelines bex controls directly, provider-side SSE for the Barman Cloud plugin-backed Postgres stores bex doesn't, and write-only per-store S3 credentials replacing today's shared `TF_STATE_ACCESS_KEY`/`SECRET_KEY` reuse across all five backup destinations. **Status:** todo

## Tasks (in order)

| id   | title                                                                              | est | depends_on               |
| ---- | ----------------------------------------------------------------------------------- | --- | ------------------------- |
| t001 | Generate + custody the `age` backup-encryption keypair                              | 30m | —                          |
| t002 | Tier A encryption: `etcd-backup` CronJob age-encrypt step                           | 30m | t001                       |
| t003 | Tier A encryption: `openbao-backup` CronJob age-encrypt step                        | 30m | t001                       |
| t004 | Tier A encryption: `kvbak-<id>` Job template age-encrypt step (operator Go)          | 45m | t001                       |
| t005 | Tier B: enable SSE (`encryption: AES256`) on the three Barman `ObjectStore` CRs      | 20m | —                          |
| t006 | Write-only/read-only IAM policy templates + `scripts/backup-s3-credentials.sh`       | 60m | —                          |
| t007 | Migrate the five backup Secrets to the new per-store writer credentials             | 45m | t006                       |
| t008 | Recovery flow: age-decrypt step in `restore-etcd.sh`/`restore-openbao.sh`/`restore-keyvalue.sh` | 45m | t002, t003, t004, t007 |
| t009 | Recovery flow: `restore-postgres.sh` reads the new reader credential, not the writer Secret | 30m | t007                |
| t010 | Live drill: encrypt→upload→fetch→decrypt→restore across all five stores             | 60m | t005, t008, t009           |
| t011 | Docs: ADR050 status update, ADR031 re-drill cadence, follow-up inbox notes           | 20m | t010                       |
| t012 | Simplify                                                                             | 20m | t011                       |
| t013 | Test coverage                                                                        | 30m | t012                       |
| t014 | Closeout                                                                             | 10m | t013                       |

## Definition of done

- All five backup destinations (etcd, OpenBao, paid KeyValue, `bex-db`, tenant Postgres, auth DBs) write through a per-store credential that cannot `GetObject` — the shared `TF_STATE_ACCESS_KEY`/`SECRET_KEY` no longer appears in any backup Secret.
- Tier A snapshots (etcd, OpenBao, KeyValue) land in `bex-tfstate` as `age`-encrypted objects; downloading one without the out-of-band private key yields ciphertext, verified live.
- The three Barman `ObjectStore` CRs report `encryption: AES256` and a fresh base backup shows the SSE header on the stored object.
- `scripts/restore-etcd.sh`, `restore-openbao.sh`, `restore-keyvalue.sh`, and `restore-postgres.sh` each complete a live `DRY_RUN=1` and a confirmed restore into a fresh `restore-*` namespace using only the new reader credential + (for Tier A) the custodied `age` private key — no restore script touches a writer Secret.
- `AGE_BACKUP_PRIVATE_KEY` is present in `.env.example` (value-less) and in `scripts/gh-secrets.sh`'s credential list; the public key is committed to git.
- ADR050's Status line points at this milestone; ADR031's re-drill cadence table reflects the new decrypt step.

## Source + Goal linkage

- **Source:** `docs/ADR050-encrypted-platform-backups.md` (Proposed), written this session from a user question about Barman Cloud Plugin backup encryption, refined through user-confirmed key-custody/credential-scoping/tiering decisions.
- **Goal linkage:** closes a secrets-hygiene gap in the same line as `docs/ADR028-security-review.md` / `docs/ADR045-security-review-round3.md` and extends the backup mechanism `docs/ADR031-platform-data-backup.md` already ships (GOAL.md #7, security review).
- **Expected outcome:** a leaked backup-job credential can no longer read back any backup's contents; Tier A backups are unreadable without a private key that was never in the cluster to begin with.
- **Why now:** the root-credential reuse across all five backup destinations was found while answering a direct user question about backup encryption — it is the platform's largest current backup-confidentiality gap and has no dependency on other in-flight work.
- **Render parity — omitted.** This milestone has no REST/GraphQL/MCP/UI surface: it changes Go operator internals, GitOps manifests, and out-of-band shell/IAM tooling only. Filed under `w5` at the user's explicit direction despite the mismatch with this workstream's Dashboard UI theme (see conversation) — there is no dashboard-facing change to review for parity.
