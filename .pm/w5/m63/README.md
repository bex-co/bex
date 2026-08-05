# w5 · m63 — Encrypted platform backups + write-scoped backup credentials (ADR050)

**Worker:** worker5 **Goal:** implement `docs/ADR050-encrypted-platform-backups.md` — client-side `age` encryption for the etcd/OpenBao/KeyValue backup pipelines bex controls directly, provider-side SSE for the Barman Cloud plugin-backed Postgres stores bex doesn't, and write-only per-store S3 credentials replacing today's shared `TF_STATE_ACCESS_KEY`/`SECRET_KEY` reuse across all five backup destinations. **Status:** shipped + deployed to prod; Tier A enabled & verified for etcd/OpenBao (credential scoping deferred)

## Prod enablement (2026-08-04)

Shipped (deploy (bex via Argo) green) and enabled Tier A **encryption** in prod per the user's "reversible encryption only" choice:

- **Tier B SSE:** live on all three ObjectStores (`encryption: AES256`), backups still `completed`.
- **Tier A etcd + OpenBao:** enabled via the out-of-band `bex-backup-age` ConfigMap + `AGE_BACKUP_PRIVATE_KEY` in `.env`/CI; manual runs produced `*.gz.age`, and a live prod `restore-etcd.sh` drill decrypted a fresh `.age` snapshot and recovered 9 CRs. Verified end-to-end.
- **Tier A KeyValue:** operator env `BEX_BACKUP_AGE_PUBLIC_KEY` set via gitops; the live `kvbak` CronJob reconciled to `[snapshot compress encrypt]`. A green KV backup is blocked by a **pre-existing** Valkey snapshot-connectivity failure unrelated to ADR050 — see `.pm/w5/039.md`.
- **Deferred (operator decision):** the write-only per-store credential rotation (§3) is built but not enabled; `t006`/`t007` remain the runbook. Follow-ups `036`/`037`/`038`; new `039` for the KV snapshot issue.

## Implementation status (2026-08-04)

All durable artifacts are implemented and locally verified (operator `go build`/`vet`/`gofmt` clean, touched controller tests + new KV encrypt tests pass, restore shellcheck clean, 15/15 hermetic restore tests pass, all edited manifests parse):

- **Landed in code:** Tier A encrypt steps in `etcd-backup`/`openbao-backup` CronJobs + the operator-templated `kvbak` Job (opt-in, off by default); Tier B `encryption: AES256` on the three Barman `ObjectStore` CRs; `infra/wasabi/backup-{writer,reader}-policy.json` + `scripts/backup-s3-credentials.sh`; restore-script age-decrypt + reader-credential wiring (incl. the required `restore-postgres.sh` reader-Secret fix); `.env.example`/`gh-secrets.sh`/`BEX_BACKUP_AGE_PUBLIC_KEY` scaffolding; ADR031 + CLAUDE.md docs; ADR050 follow-ups filed as `036`/`037`/`038`.
- **Pending an operator (cluster + Wasabi credentials — cannot be done from a dev box):** t001 real `age-keygen` into prod `.env`/CI + `bex-backup-age` ConfigMaps; t006/t007 live IAM `provision`/`verify` + writer-Secret migration + `revoke-legacy`; t010 the live encrypt→upload→fetch→decrypt→restore drill; t011's ADR050 `Proposed`→`Accepted` flip (gated on the drill).

Everything is **opt-in / off by default**, so the shipped code is byte-identical to pre-ADR050 until an operator enables it — safe to merge ahead of the live enablement.

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
