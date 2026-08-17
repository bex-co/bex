#!/usr/bin/env bash
# Restore an OpenBao Raft snapshot into a newly-created one-node throwaway.
# There is deliberately no same-instance/live mode: this tool cannot overwrite
# the production OpenBao service or its PVCs.
set -euo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$HERE/.." && pwd)"
# shellcheck source=scripts/lib/restore.sh
source "$HERE/lib/restore.sh"

usage() {
  cat <<'EOF'
Usage:
  DRY_RUN=1 scripts/restore-openbao.sh --target-namespace restore-NAME \
    --verify-path tenants/data/PATH \
    [--snapshot latest|s3://bucket/key.snap.gz] [--image IMAGE]

  scripts/restore-openbao.sh <same options> --confirm restore-NAME \
    [--teardown-on-success]

  scripts/restore-openbao.sh --teardown restore-NAME --confirm restore-NAME

Actual recovery requires BAO_UNSEAL_KEY_1/2/3 and BAO_ROOT_TOKEN in the
environment or gitignored .env. They are sent through request bodies/config
stdin, never output or argv. Only the fresh-node snapshot-force API is used.
EOF
}

TARGET_NAMESPACE=""
VERIFY_PATH=""
SNAPSHOT="latest"
IMAGE="quay.io/openbao/openbao:2.5.5@sha256:6150c4a6b62067db6141c8da7a6a6b5763f4f47c315343d0c848b40fecdfd452"
CONFIRM=""
TEARDOWN=""
TEARDOWN_ON_SUCCESS=0

while [ "$#" -gt 0 ]; do
  case "$1" in
    --target-namespace) TARGET_NAMESPACE="${2:-}"; shift 2 ;;
    --verify-path) VERIFY_PATH="${2:-}"; shift 2 ;;
    --snapshot) SNAPSHOT="${2:-}"; shift 2 ;;
    --image) IMAGE="${2:-}"; shift 2 ;;
    --confirm) CONFIRM="${2:-}"; shift 2 ;;
    --teardown) TEARDOWN="${2:-}"; shift 2 ;;
    --teardown-on-success) TEARDOWN_ON_SUCCESS=1; shift ;;
    --live|--same-instance) restore_die "live/in-place OpenBao restore is forbidden; use a restore-* target" ;;
    -h|--help) usage; exit 0 ;;
    *) restore_die "unknown argument: $1" ;;
  esac
done

restore_require_command kubectl
restore_require_command curl
restore_require_command jq

if [ -n "$TEARDOWN" ]; then
  [ -z "$TARGET_NAMESPACE$VERIFY_PATH" ] || restore_die "--teardown cannot be combined with recovery options"
  restore_require_throwaway_namespace "$TEARDOWN"
  restore_require_confirmation "$TEARDOWN" "$CONFIRM"
  [ "${DRY_RUN:-0}" != "1" ] || restore_die "teardown is unavailable in DRY_RUN mode"
  restore_delete_namespace "$TEARDOWN"
  exit 0
fi

[ -n "$TARGET_NAMESPACE" ] || restore_die "missing --target-namespace"
[ -n "$VERIFY_PATH" ] || restore_die "missing --verify-path"
restore_require_throwaway_namespace "$TARGET_NAMESPACE"
[[ "$VERIFY_PATH" == tenants/* ]] || restore_die "--verify-path must identify a tenants/ KV path"
[[ "$VERIFY_PATH" != *$'\n'* && "$VERIFY_PATH" != *'..'* ]] || restore_die "invalid verification path"
restore_require_digest_image "$IMAGE" "OpenBao image"

restore_load_dotenv "$REPO_ROOT"
restore_prefer_reader_credential openbao
bucket="${RESTORE_S3_BUCKET_NAME:-${TF_STATE_BUCKET:-}}"
[ -n "$bucket" ] || restore_die "TF_STATE_BUCKET (or RESTORE_S3_BUCKET_NAME) is required"
restore_resolve_snapshot "$SNAPSHOT" "s3://$bucket/openbao-snapshots/" ".snap.gz"

scratch="$(mktemp -d "${TMPDIR:-/tmp}/bex-bao-restore.XXXXXX")"
BAO_PF_PID=""
cleanup() {
  if [ -n "$BAO_PF_PID" ]; then
    kill "$BAO_PF_PID" 2>/dev/null || true
    wait "$BAO_PF_PID" 2>/dev/null || true
  fi
  rm -rf "$scratch"
}
trap cleanup EXIT

download="$scratch/download"
archive="$scratch/snapshot.snap.gz"
snapshot_file="$scratch/snapshot.snap"
restore_fetch_snapshot "$RESTORE_SNAPSHOT_URI" "$download"
restore_decrypt_if_age "$RESTORE_SNAPSHOT_URI" "$download" "$archive"
restore_gunzip_checked "$archive" "$snapshot_file"

render_target() {
  cat <<EOF
apiVersion: v1
kind: ConfigMap
metadata:
  name: openbao-restore-config
  namespace: $TARGET_NAMESPACE
  labels: {bex.co/restore-target: "true"}
data:
  restore.hcl: |
    ui = false
    disable_mlock = true
    api_addr = "http://openbao-0.openbao-internal:8200"
    cluster_addr = "https://openbao-0.openbao-internal:8201"
    listener "tcp" {
      tls_disable = 1
      address = "[::]:8200"
      cluster_address = "[::]:8201"
    }
    storage "raft" {
      path = "/openbao/data"
    }
---
apiVersion: v1
kind: Service
metadata:
  name: openbao-internal
  namespace: $TARGET_NAMESPACE
  labels: {bex.co/restore-target: "true"}
spec:
  clusterIP: None
  publishNotReadyAddresses: true
  selector: {app: openbao-restore}
  ports:
    - {name: http, port: 8200, targetPort: 8200}
    - {name: raft, port: 8201, targetPort: 8201}
---
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: openbao-restore-deny-all
  namespace: $TARGET_NAMESPACE
  labels: {bex.co/restore-target: "true"}
spec:
  podSelector:
    matchLabels: {app: openbao-restore}
  policyTypes: [Ingress, Egress]
---
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: openbao
  namespace: $TARGET_NAMESPACE
  labels: {bex.co/restore-target: "true"}
spec:
  serviceName: openbao-internal
  replicas: 1
  selector:
    matchLabels: {app: openbao-restore}
  template:
    metadata:
      labels: {app: openbao-restore, bex.co/restore-target: "true"}
    spec:
      nodeSelector: {bex.co/pool: platform}
      tolerations:
        - {key: bex.co/platform, operator: Equal, value: "true", effect: NoSchedule}
      securityContext:
        runAsNonRoot: true
        runAsUser: 100
        runAsGroup: 1000
        fsGroup: 1000
        seccompProfile: {type: RuntimeDefault}
      containers:
        - name: openbao
          image: $IMAGE
          imagePullPolicy: IfNotPresent
          command: [/bin/sh, -ec]
          args: ["exec bao server -config=/openbao/config/restore.hcl"]
          env:
            - {name: BAO_ADDR, value: "http://127.0.0.1:8200"}
            - {name: SKIP_CHOWN, value: "true"}
            - {name: SKIP_SETCAP, value: "true"}
            - {name: HOME, value: /home/openbao}
          ports:
            - {name: http, containerPort: 8200}
            - {name: raft, containerPort: 8201}
          readinessProbe:
            exec:
              command: [/bin/sh, -ec, 'code=0; bao status >/dev/null 2>&1 || code=\$?; [ "\$code" -eq 0 ] || [ "\$code" -eq 2 ]']
            periodSeconds: 2
            timeoutSeconds: 2
          resources:
            requests: {cpu: 50m, memory: 64Mi}
            limits: {cpu: 250m, memory: 256Mi}
          securityContext:
            allowPrivilegeEscalation: false
            capabilities: {drop: [ALL]}
          volumeMounts:
            - {name: data, mountPath: /openbao/data}
            - {name: config, mountPath: /openbao/config}
            - {name: home, mountPath: /home/openbao}
      volumes:
        - name: config
          configMap: {name: openbao-restore-config}
        - name: home
          emptyDir: {}
  volumeClaimTemplates:
    - metadata:
        name: data
        labels: {bex.co/restore-target: "true"}
      spec:
        accessModes: [ReadWriteOnce]
        storageClassName: "${RESTORE_STORAGE_CLASS:-hcloud-volumes}"
        resources:
          requests: {storage: "${RESTORE_BAO_STORAGE:-2Gi}"}
EOF
}

echo "OpenBao recovery plan"
echo "  source: $RESTORE_SNAPSHOT_URI"
echo "  target: $TARGET_NAMESPACE/openbao-0 (new namespace and new PVC only)"
# ADR050 §Recovery flow: OpenBao restore is three secrets deep, applied in order.
echo "  secrets (in order): reader S3 credential (fetch) -> AGE_BACKUP_PRIVATE_KEY"
echo "                      (unwrap the .age transport layer, if encrypted) ->"
echo "                      original BAO_UNSEAL_KEY_1/2/3 (unseal the restored Raft)"
echo "  transition: new init -> fresh unseal -> snapshot-force -> restart -> OLD-key unseal -> tenants/ verification"
echo "  verification path and all key/token values are suppressed"
render_target
restore_print_dry_run
if [ "${DRY_RUN:-0}" = "1" ]; then
  exit 0
fi

restore_require_confirmation "$TARGET_NAMESPACE" "$CONFIRM"
for name in BAO_UNSEAL_KEY_1 BAO_UNSEAL_KEY_2 BAO_UNSEAL_KEY_3 BAO_ROOT_TOKEN; do
  [ -n "${!name:-}" ] || restore_die "$name is required in env or .env"
done
if kubectl get namespace "$TARGET_NAMESPACE" >/dev/null 2>&1; then
  restore_die "target namespace already exists; choose a new restore-* namespace"
fi
restore_create_namespace "$TARGET_NAMESPACE" openbao
render_target | kubectl apply -f - >/dev/null
kubectl -n "$TARGET_NAMESPACE" rollout status statefulset/openbao \
  --timeout="${RESTORE_READY_TIMEOUT:-10m}" >/dev/null

start_forward() {
  local log="$scratch/port-forward.log"
  : >"$log"
  kubectl -n "$TARGET_NAMESPACE" port-forward pod/openbao-0 \
    "${BAO_RESTORE_LOCAL_PORT:-38250}:8200" >"$log" 2>&1 &
  BAO_PF_PID="$!"
  BAO_RESTORE_ADDR="http://127.0.0.1:${BAO_RESTORE_LOCAL_PORT:-38250}"
  for _ in $(seq 1 60); do
    if curl -fsS "$BAO_RESTORE_ADDR/v1/sys/seal-status" | jq -e 'has("sealed")' >/dev/null 2>&1; then
      return
    fi
    sleep 1
  done
  restore_die "throwaway OpenBao did not become reachable"
}

stop_forward() {
  if [ -n "$BAO_PF_PID" ]; then
    kill "$BAO_PF_PID" 2>/dev/null || true
    wait "$BAO_PF_PID" 2>/dev/null || true
    BAO_PF_PID=""
  fi
}

unseal_with_key() {
  local key="$1"
  printf '{"key":"%s"}' "$key" | \
    curl -fsS -X PUT "$BAO_RESTORE_ADDR/v1/sys/unseal" --data-binary @-
}

token_config() {
  local token="$1"
  printf 'header = "X-Vault-Token: %s"\n' "$token"
}

started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
start_forward
umask 077
curl -fsS -X PUT "$BAO_RESTORE_ADDR/v1/sys/init" \
  --data-binary '{"secret_shares":1,"secret_threshold":1}' >"$scratch/fresh-init.json"
fresh_key="$(jq -er '.keys_base64[0]' "$scratch/fresh-init.json")"
fresh_token="$(jq -er '.root_token' "$scratch/fresh-init.json")"
[ "$(unseal_with_key "$fresh_key" | jq -r .sealed)" = "false" ] || \
  restore_die "fresh target did not unseal"

token_config "$fresh_token" | curl -fsS --config - -X POST \
  --data-binary "@$snapshot_file" "$BAO_RESTORE_ADDR/v1/sys/storage/raft/snapshot-force" >/dev/null
unset fresh_key fresh_token

stop_forward
kubectl -n "$TARGET_NAMESPACE" delete pod openbao-0 --wait=true >/dev/null
kubectl -n "$TARGET_NAMESPACE" rollout status statefulset/openbao \
  --timeout="${RESTORE_READY_TIMEOUT:-10m}" >/dev/null
start_forward

unseal_with_key "$BAO_UNSEAL_KEY_1" >/dev/null
unseal_with_key "$BAO_UNSEAL_KEY_2" >/dev/null
[ "$(unseal_with_key "$BAO_UNSEAL_KEY_3" | jq -r .sealed)" = "false" ] || \
  restore_die "restored target remained sealed after original keys"
token_config "$BAO_ROOT_TOKEN" | curl -fsS --config - -o /dev/null \
  "$BAO_RESTORE_ADDR/v1/$VERIFY_PATH"
ready_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
echo "snapshot restore, original-key unseal, and tenants/ verification passed"
echo "recovery ran from $started_at to $ready_at"

stop_forward
if [ "$TEARDOWN_ON_SUCCESS" -eq 1 ]; then
  restore_delete_namespace "$TARGET_NAMESPACE"
else
  echo "target retained for review; teardown with: scripts/restore-openbao.sh --teardown $TARGET_NAMESPACE --confirm $TARGET_NAMESPACE"
fi
