# Static-site S3 credential rotation

This runbook rotates the two bucket-scoped static-site identities without putting credential values in Git, shell arguments, logs, or evidence. It assumes `bex-static` already exists and `TF_STATE_*` in the gitignored `.env` is the out-of-band Wasabi IAM administrator. Provider users are `bex-static-reader` and `bex-static-publisher`; Kubernetes consumers are `bex-system/bex-static-read-s3` and `bex-build/bex-static-publish-s3`.

The Terraform-state credential is not a runtime fallback. It remains in out-of-band custody because Terraform and the backup plane still use `bex-tfstate`, but after this migration no static workload mounts its old `static-s3` copy.

## Add and verify

Use a mode-0600 production kubeconfig outside the repository. The helper inherits secrets into the pinned AWS CLI container and sends Kubernetes Secret values over stdin; it prints names and allow/deny results only.

```sh
KUBECONFIG=/secure/path/app.kubeconfig scripts/static-s3-credentials.sh provision
KUBECONFIG=/secure/path/app.kubeconfig scripts/static-s3-credentials.sh verify
```

The verification must show:

- publisher put/read/delete on `bex-static`: allowed;
- reader list/read on `bex-static`: allowed;
- reader put/delete: denied;
- both identities listing `bex-tfstate` (the etcd/OpenBao/CNPG backup bucket): denied;
- both identities listing an unrelated account bucket: denied.

Stop if any expected denial succeeds. Do not deploy a broader policy to make the test pass.

## Deploy and prove

Ship through the normal signed `deploy (bex via Argo)` workflow. It rolls the operator and static-server to the contracts in [ADR029](../ADR029-static-sites.md):

- manager `BEX_STATIC_PUBLISH_S3_SECRET=bex-static-publish-s3` in `bex-build`;
- static-server `envFrom.secretRef=bex-static-read-s3` in `bex-system`;
- static-server startup list check and pre-Job publisher-Secret validation.

During rollout, the already-published revision stays in the private bucket. If the new reader cannot start, restore the previous deployment digest and its `static-s3` reference before touching the old Secret. If the manager reports `StaticCredentialUnavailable`, restore its prior digest/config and leave the new identities active but unused.

After both workloads are Ready:

1. Fetch an existing `https://<site>.onbex.co` and record only status/byte count.
2. Trigger a fresh static deploy and wait for a new revision to reach `Running`.
3. Fetch the new revision through the same public hostname.
4. Create a disposable static site, publish it, then delete it; prove only that App/revision prefix disappears while the original site remains readable.
5. Run `scripts/verify-static-site-security.sh live` and the S3 matrix again.

Never capture deploy-hook tokens, request authorization headers, Secret payloads, access-key ids, kubeconfig content, pod environment, or command traces with `set -x`.

## Revoke the legacy static-plane copy

The helper refuses removal while any Pod, Deployment, StatefulSet, DaemonSet, Job, or CronJob still references `static-s3`, and re-runs the permission matrix first:

```sh
KUBECONFIG=/secure/path/app.kubeconfig scripts/static-s3-credentials.sh revoke-legacy
```

It deletes only the legacy Kubernetes Secrets from `bex-system`, `bex-build`, and `default`. That is the revocation boundary: the account-wide Terraform/backup key is no longer obtainable by a compromised static component. The root key itself must not be disabled until every Terraform and backup consumer has separately rotated.

Confirm absence without reading payloads:

```sh
kubectl get secret -A --field-selector metadata.name=static-s3
kubectl get pods,deployments,statefulsets,daemonsets,jobs,cronjobs -A -o json \
  | jq -e '.. | objects | select(.secretRef?.name == "static-s3" or .secret?.secretName == "static-s3")'
```

Both commands should find nothing (the second exits non-zero). Re-run the public static fetch and S3 matrix after removal.

## Later rotations

Wasabi permits two active keys per IAM user, which provides the rollback window:

1. create a second key for one IAM user;
2. apply it to only that user's namespace-local Secret without logging it;
3. roll only that consumer and repeat its positive/negative matrix;
4. deactivate, verify, then delete the previous provider key;
5. repeat for the other user.

Never rotate reader and publisher simultaneously. A reader rollback must not grant write; a publisher rollback must not restore access to `bex-tfstate`.
