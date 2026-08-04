#!/usr/bin/env bash
# Restore a paid KeyValue RDB into a new, isolated Valkey instance. The script
# has no in-place mode: the target namespace and PVC are always newly created.
set -euo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$HERE/.." && pwd)"
# shellcheck source=scripts/lib/restore.sh
source "$HERE/lib/restore.sh"

usage() {
  cat <<'EOF'
Usage:
  DRY_RUN=1 scripts/restore-keyvalue.sh --id red-ID \
    --target-namespace restore-NAME --verify-key KEY [--expect VALUE] \
    [--snapshot latest|s3://bucket/key.rdb.gz] [--image valkey/valkey:TAG]

  scripts/restore-keyvalue.sh <same options> --confirm restore-NAME \
    [--teardown-on-success]

  scripts/restore-keyvalue.sh --teardown restore-NAME --confirm restore-NAME

The selected RDB is checksum-validated, seeded only onto an empty PVC, loaded
once with AOF disabled, then rewritten as AOF and verified after a rollout.
Passwords are generated for the throwaway and never appear in output or argv.
EOF
}

ID=""
TARGET_NAMESPACE=""
VERIFY_KEY=""
EXPECT=""
EXPECT_SET=0
SNAPSHOT="latest"
IMAGE="valkey/valkey:8-alpine"
CONFIRM=""
TEARDOWN=""
TEARDOWN_ON_SUCCESS=0

while [ "$#" -gt 0 ]; do
  case "$1" in
    --id) ID="${2:-}"; shift 2 ;;
    --target-namespace) TARGET_NAMESPACE="${2:-}"; shift 2 ;;
    --verify-key) VERIFY_KEY="${2:-}"; shift 2 ;;
    --expect) EXPECT="${2:-}"; EXPECT_SET=1; shift 2 ;;
    --snapshot) SNAPSHOT="${2:-}"; shift 2 ;;
    --image) IMAGE="${2:-}"; shift 2 ;;
    --confirm) CONFIRM="${2:-}"; shift 2 ;;
    --teardown) TEARDOWN="${2:-}"; shift 2 ;;
    --teardown-on-success) TEARDOWN_ON_SUCCESS=1; shift ;;
    -h|--help) usage; exit 0 ;;
    *) restore_die "unknown argument: $1" ;;
  esac
done

restore_require_command kubectl
restore_require_command docker
restore_require_command openssl

if [ -n "$TEARDOWN" ]; then
  [ -z "$ID$TARGET_NAMESPACE$VERIFY_KEY" ] || restore_die "--teardown cannot be combined with recovery options"
  restore_require_throwaway_namespace "$TEARDOWN"
  restore_require_confirmation "$TEARDOWN" "$CONFIRM"
  [ "${DRY_RUN:-0}" != "1" ] || restore_die "teardown is unavailable in DRY_RUN mode"
  restore_delete_namespace "$TEARDOWN"
  exit 0
fi

[ -n "$ID" ] || restore_die "missing --id"
[ -n "$TARGET_NAMESPACE" ] || restore_die "missing --target-namespace"
[ -n "$VERIFY_KEY" ] || restore_die "missing --verify-key"
restore_validate_dns_label "$ID" "KeyValue id"
[[ "$ID" == red-* ]] || restore_die "KeyValue id must start with red-"
restore_require_throwaway_namespace "$TARGET_NAMESPACE"
[[ "$VERIFY_KEY" != *$'\n'* ]] || restore_die "verification key must be one line"
[[ "$IMAGE" =~ ^[A-Za-z0-9./:@_-]+$ ]] || restore_die "invalid Valkey image"

restore_load_dotenv "$REPO_ROOT"
restore_prefer_reader_credential keyvalue
bucket="${RESTORE_S3_BUCKET_NAME:-${TF_STATE_BUCKET:-}}"
[ -n "$bucket" ] || restore_die "TF_STATE_BUCKET (or RESTORE_S3_BUCKET_NAME) is required"
restore_resolve_snapshot "$SNAPSHOT" "s3://$bucket/keyvalue/$ID/" ".rdb.gz"

scratch="$(mktemp -d "${TMPDIR:-/tmp}/bex-kv-restore.XXXXXX")"
trap 'rm -rf "$scratch"' EXIT
download="$scratch/download"
archive="$scratch/snapshot.rdb.gz"
rdb="$scratch/dump.rdb"
restore_fetch_snapshot "$RESTORE_SNAPSHOT_URI" "$download"
restore_decrypt_if_age "$RESTORE_SNAPSHOT_URI" "$download" "$archive"
restore_gunzip_checked "$archive" "$rdb"
docker run --rm -v "$scratch:/restore:ro" --entrypoint /usr/local/bin/valkey-check-rdb \
  "$IMAGE" /restore/dump.rdb >/dev/null || restore_die "Valkey RDB checksum validation failed"

render_workload() {
  local appendonly="$1"
  cat <<EOF
apiVersion: v1
kind: Service
metadata:
  name: restore-kv
  namespace: $TARGET_NAMESPACE
  labels: {bex.co/restore-target: "true"}
spec:
  clusterIP: None
  selector: {app: restore-kv}
  ports:
    - {name: valkey, port: 6379, targetPort: 6379}
---
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: restore-kv
  namespace: $TARGET_NAMESPACE
  labels: {bex.co/restore-target: "true"}
spec:
  serviceName: restore-kv
  replicas: 1
  selector:
    matchLabels: {app: restore-kv}
  template:
    metadata:
      labels: {app: restore-kv, bex.co/restore-target: "true"}
    spec:
      securityContext:
        runAsNonRoot: true
        runAsUser: 999
        runAsGroup: 1000
        fsGroup: 1000
        seccompProfile: {type: RuntimeDefault}
      containers:
        - name: valkey
          image: $IMAGE
          imagePullPolicy: IfNotPresent
          command: [/bin/sh, -ec]
          args:
            - |
              umask 077
              printf 'dir /data\nsave ""\nappendonly $appendonly\nrequirepass %s\n' "\$VALKEY_PASSWORD" >/tmp/restore-valkey.conf
              exec valkey-server /tmp/restore-valkey.conf
          env:
            - name: VALKEY_PASSWORD
              valueFrom:
                secretKeyRef: {name: restore-kv-auth, key: password}
          ports:
            - {name: valkey, containerPort: 6379}
          readinessProbe:
            exec:
              command: [/bin/sh, -ec, 'REDISCLI_AUTH="\$VALKEY_PASSWORD" valkey-cli ping | grep -qx PONG']
            periodSeconds: 2
            timeoutSeconds: 2
          resources:
            requests: {cpu: 25m, memory: 32Mi}
            limits: {cpu: 250m, memory: 256Mi}
          securityContext:
            allowPrivilegeEscalation: false
            capabilities: {drop: [ALL]}
          volumeMounts:
            - {name: data, mountPath: /data}
      volumes:
        - name: data
          persistentVolumeClaim: {claimName: restore-kv-data}
EOF
}

echo "KeyValue recovery plan"
echo "  source: $RESTORE_SNAPSHOT_URI"
echo "  target: $TARGET_NAMESPACE/restore-kv (new namespace and new PVC only)"
echo "  transition: checksum -> empty PVC -> AOF off -> key check -> AOF rewrite -> restart -> key check"
echo "  verification key/value are not printed"
render_workload no
restore_print_dry_run
if [ "${DRY_RUN:-0}" = "1" ]; then
  exit 0
fi

restore_require_confirmation "$TARGET_NAMESPACE" "$CONFIRM"
if kubectl get namespace "$TARGET_NAMESPACE" >/dev/null 2>&1; then
  restore_die "target namespace already exists; choose a new restore-* namespace"
fi
restore_create_namespace "$TARGET_NAMESPACE" keyvalue

password="$(openssl rand -hex 24)"
cat <<EOF | kubectl apply -f - >/dev/null
apiVersion: v1
kind: Secret
metadata:
  name: restore-kv-auth
  namespace: $TARGET_NAMESPACE
  labels: {bex.co/restore-target: "true"}
type: Opaque
stringData:
  password: "$password"
---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: restore-kv-data
  namespace: $TARGET_NAMESPACE
  labels: {bex.co/restore-target: "true"}
spec:
  accessModes: [ReadWriteOnce]
  storageClassName: "${RESTORE_STORAGE_CLASS:-hcloud-volumes}"
  resources:
    requests: {storage: "${RESTORE_KV_STORAGE:-1Gi}"}
---
apiVersion: v1
kind: Pod
metadata:
  name: restore-kv-seed
  namespace: $TARGET_NAMESPACE
  labels: {bex.co/restore-target: "true"}
spec:
  restartPolicy: Never
  securityContext:
    runAsNonRoot: true
    runAsUser: 999
    runAsGroup: 1000
    fsGroup: 1000
    seccompProfile: {type: RuntimeDefault}
  containers:
    - name: seed
      image: $IMAGE
      command: [/bin/sh, -ec, "sleep 3600"]
      securityContext:
        allowPrivilegeEscalation: false
        capabilities: {drop: [ALL]}
      volumeMounts:
        - {name: data, mountPath: /data}
  volumes:
    - name: data
      persistentVolumeClaim: {claimName: restore-kv-data}
EOF

kubectl -n "$TARGET_NAMESPACE" wait --for=condition=Ready pod/restore-kv-seed \
  --timeout="${RESTORE_READY_TIMEOUT:-10m}" >/dev/null
kubectl -n "$TARGET_NAMESPACE" exec restore-kv-seed -- /bin/sh -ec \
  'test ! -e /data/dump.rdb && test ! -e /data/appendonly.aof && test ! -e /data/appendonlydir'
# kubectl exec's stdin transport can terminate after creating an empty file on
# some API-server/client combinations. kubectl cp uses tar on the same isolated
# Pod and fails atomically enough for the mandatory non-empty check below.
kubectl -n "$TARGET_NAMESPACE" cp "$rdb" restore-kv-seed:/data/dump.rdb -c seed
kubectl -n "$TARGET_NAMESPACE" exec restore-kv-seed -- /bin/sh -ec \
  'test -s /data/dump.rdb && test ! -e /data/appendonly.aof && test ! -e /data/appendonlydir'
kubectl -n "$TARGET_NAMESPACE" delete pod restore-kv-seed --wait=true >/dev/null

render_workload no | kubectl apply -f - >/dev/null
started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
kubectl -n "$TARGET_NAMESPACE" rollout status statefulset/restore-kv \
  --timeout="${RESTORE_READY_TIMEOUT:-10m}" >/dev/null

kv_cli() {
  # Expansion happens in the remote container, not in this operator shell.
  # shellcheck disable=SC2016
  kubectl -n "$TARGET_NAMESPACE" exec restore-kv-0 -- /bin/sh -ec \
    'REDISCLI_AUTH="$VALKEY_PASSWORD" valkey-cli --raw "$@"' -- "$@"
}

value="$(kv_cli GET "$VERIFY_KEY")"
[ -n "$value" ] || restore_die "verification key was absent after RDB load"
if [ "$EXPECT_SET" -eq 1 ]; then
  [ "$value" = "$EXPECT" ] || restore_die "verification value did not match --expect"
fi
[ "$(kv_cli INFO persistence | tr -d '\r' | awk -F: '$1 == "aof_enabled" {print $2}')" = "0" ] || \
  restore_die "AOF unexpectedly enabled during RDB seed boot"
echo "RDB marker verification passed with AOF disabled (value suppressed)"

kv_cli CONFIG SET appendonly yes >/dev/null
# This entire single-quoted program runs inside the throwaway Pod.
# shellcheck disable=SC2016
kubectl -n "$TARGET_NAMESPACE" exec restore-kv-0 -- /bin/sh -ec '
  for i in $(seq 1 120); do
    state=$(REDISCLI_AUTH="$VALKEY_PASSWORD" valkey-cli --raw INFO persistence | tr -d "\r")
    [ "$(printf "%s\n" "$state" | awk -F: '\''$1 == "aof_rewrite_in_progress" {print $2}'\'')" = 0 ] && exit 0
    sleep 1
  done
  exit 1
'
kv_cli BGREWRITEAOF >/dev/null
# This entire single-quoted program runs inside the throwaway Pod.
# shellcheck disable=SC2016
kubectl -n "$TARGET_NAMESPACE" exec restore-kv-0 -- /bin/sh -ec '
  for i in $(seq 1 120); do
    state=$(REDISCLI_AUTH="$VALKEY_PASSWORD" valkey-cli --raw INFO persistence | tr -d "\r")
    [ "$(printf "%s\n" "$state" | awk -F: '\''$1 == "aof_rewrite_in_progress" {print $2}'\'')" = 0 ] && exit 0
    sleep 1
  done
  exit 1
'

render_workload yes | kubectl apply -f - >/dev/null
kubectl -n "$TARGET_NAMESPACE" rollout status statefulset/restore-kv \
  --timeout="${RESTORE_READY_TIMEOUT:-10m}" >/dev/null
value="$(kv_cli GET "$VERIFY_KEY")"
[ -n "$value" ] || restore_die "verification key was absent after AOF restart"
if [ "$EXPECT_SET" -eq 1 ]; then
  [ "$value" = "$EXPECT" ] || restore_die "post-restart value did not match --expect"
fi
[ "$(kv_cli INFO persistence | tr -d '\r' | awk -F: '$1 == "aof_enabled" {print $2}')" = "1" ] || \
  restore_die "AOF was not enabled after restart"
ready_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
echo "AOF rewrite and post-restart marker verification passed (value suppressed)"
echo "recovery ran from $started_at to $ready_at"

unset password value
if [ "$TEARDOWN_ON_SUCCESS" -eq 1 ]; then
  restore_delete_namespace "$TARGET_NAMESPACE"
else
  echo "target retained for review; teardown with: scripts/restore-keyvalue.sh --teardown $TARGET_NAMESPACE --confirm $TARGET_NAMESPACE"
fi
