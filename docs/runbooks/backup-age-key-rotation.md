# Platform backup age-key rotation

This is the rotation procedure for ADR050 Tier A: etcd, OpenBao, and paid KeyValue backups. It does not rotate S3 credentials, OpenBao unseal keys, Barman/Postgres SSE, or the separate disk-snapshot age keypair (ADR082). The private identities stay in operator `.env` / GitHub Actions custody; only public recipients enter Git or the cluster.

## Rotation contract

Use one **new** recipient for new backups. Do not put two recipients into `AGE_PUBLIC_KEY` or `BEX_BACKUP_AGE_PUBLIC_KEY`: the current jobs pass that value as one `age -r` argument, not a recipient list. Dual-recipient encryption would also keep new backups decryptable by the retired key, without changing the need to retain that key for old objects. Two-generation overlap belongs on the restore side instead.

`scripts/lib/restore.sh` writes `AGE_BACKUP_PRIVATE_KEY` plus optional `AGE_BACKUP_PRIVATE_KEY_PREVIOUS` into one mode-0600 identity file. Native age tries both identities; neither key enters command arguments or cluster objects. The same file is used by the digest-pinned container fallback. A failed decrypt still fails the restore; plaintext inputs retain their existing behavior.

Do not start another rotation while `_PREVIOUS` protects retained objects. This procedure supports two generations; additional generations need separately custodied identity files until their objects expire. For a compromised key, cut over promptly but retain its recovery copy securely: rotation does not retroactively protect old ciphertext from someone who already copied the key.

## 1. Prepare custody before changing writers

Run from the repository root in a trusted shell with tracing disabled. Read the current private key from its existing custody; do not print `.env`, key files, or shell variables. GitHub does not allow reading secret values back with `gh`. If the local custody copy is unavailable, perform the work in the trusted custody environment rather than replacing an unknown current key.

```sh
set +x
umask 077
rotation_dir=$(mktemp -d)
age-keygen -o "$rotation_dir/new.identity"
age-keygen -y "$rotation_dir/new.identity" > "$rotation_dir/new.recipient"
```

Securely edit the local, gitignored `.env`:

- Copy the current `AGE_BACKUP_PRIVATE_KEY` value into `AGE_BACKUP_PRIVATE_KEY_PREVIOUS` before replacing anything.
- Set `AGE_BACKUP_PRIVATE_KEY` from `new.identity` (the `AGE-SECRET-KEY-...` line).
- Set both `AGE_BACKUP_PUBLIC_KEY` and `BEX_BACKUP_AGE_PUBLIC_KEY` to `new.recipient` (public, not secret).

Mirror custody with `bash scripts/gh-secrets.sh`. That script sends private values over stdin, including `_PREVIOUS`, without printing them. Both `openbao-restore-drill.yml` and `keyvalue-restore-drill.yml` forward the optional previous identity to their decrypt helper. Keep the temporary directory protected through the cutover and verification steps below, then remove it once custody and restores are verified.

Before switching writers, download a known old encrypted backup through its reader credential and prove `age -d -i` with a protected file containing the previous identity can decrypt it. Record only object URI, timestamp, and outcome. Do not record plaintext or private-key values in drill evidence.

## 2. Cut over every public recipient

The enabled surfaces are:

| Writer | Recipient source |
| --- | --- |
| etcd | `kube-system/bex-backup-age` ConfigMap, `AGE_PUBLIC_KEY` |
| OpenBao | `secrets/bex-backup-age` ConfigMap, `AGE_PUBLIC_KEY` |
| paid KeyValue | operator `BEX_BACKUP_AGE_PUBLIC_KEY`, committed in `lego/operator/config/prod/kustomization.yaml` |

Load the new **public** recipient into the command environment from its custodied file, then replace the two ConfigMaps using the explicit target context:

```sh
AGE_BACKUP_PUBLIC_KEY=$(cat "$rotation_dir/new.recipient")
for namespace in kube-system secrets; do
  kubectl --context "$BACKUP_KUBE_CONTEXT" -n "$namespace" create configmap bex-backup-age \
    --from-literal=AGE_PUBLIC_KEY="$AGE_BACKUP_PUBLIC_KEY" --dry-run=client -o yaml |
    kubectl --context "$BACKUP_KUBE_CONTEXT" apply -f -
done
```

Replace the public recipient in the production operator overlay, review the diff, and use the repository's authorized ship/deploy workflow. Verify the running operator deployment's public environment value and its reconciled `kvbak-*` CronJobs. Editing `.env` alone does not update the committed overlay or cluster. Do not disable encryption during cutover.

Let already-running backup jobs finish and record the final old-recipient backup timestamp across **all three** writers. Job pods created before the cutover retain their old environment even after a ConfigMap changes. The retirement boundary begins after the last such job, not the first key edit.

## 3. Verify old and new restores

Trigger or observe a successful new encrypted backup for etcd, OpenBao, and at least one paid KeyValue instance. Retain an explicit pre-cutover object URI for each store; `latest` alone cannot prove overlap.

For both the pre-cutover and post-cutover object of each store, use its existing `scripts/restore-*.sh` invocation with `DRY_RUN=1`, an explicit `--snapshot` URI, and a fresh `restore-*` target namespace. ADR031 contains each store's required verification arguments. The current-plus-previous keyring must decrypt both without changing their gzip/checksum validation. Verify one new object with **only the new identity** as well, so a stale writer cannot pass merely because the combined keyring contains the old identity. The OpenBao restore still needs its original unseal shares after removing the age transport wrapper.

Record the rollout revision, successful object URIs/timestamps, reader identities (names only), and old/new decrypt outcomes. A local helper test is not evidence that production writers or CI custody changed.

## 4. Retire the previous identity by inventory

ADR031 currently retains **seven snapshots per Tier A prefix**, not a guaranteed seven-day lifetime. Failed, suspended, or inactive writers may leave old objects indefinitely; copied archives or retained object versions can extend that further. Postgres's 30-day retention belongs to Tier B and does not determine this key's retirement date.

List every etcd/OpenBao/KeyValue backup prefix with the reader credential. Confirm every old-recipient object has expired, or retain its key as long as that object is intentionally retained. Include offline copies and any retained versions in the custody inventory. Do not delete old backups just to meet a rotation date. A calendar wait without an inventory is insufficient.

Only after the inventory and new-key-only restore drill pass:

1. Clear `AGE_BACKUP_PRIVATE_KEY_PREVIOUS` in `.env` and remove its old value from any other custody copy.
2. Explicitly delete the GitHub secret: `gh secret delete AGE_BACKUP_PRIVATE_KEY_PREVIOUS` (include `--env` if it was also installed in an environment). `gh-secrets.sh` skips empty values; it does **not** delete a previously stored secret.
3. Remove temporary identity/decrypted files and record the retirement timestamp. Keep the public recipient and non-secret rotation evidence for audit.

## Rollback

Before retiring the old identity, a writer rollout can be reverted to its old public recipient. Keep both private identities available to restore objects from either side of that rollback. Recompute the last old-recipient timestamp and inventory before attempting retirement again; do not overwrite `_PREVIOUS` with another generation during rollback.
