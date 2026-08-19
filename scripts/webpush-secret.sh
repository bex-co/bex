#!/usr/bin/env bash
# Mint or install the browser Web Push VAPID keypair as bex-system/bex-webpush.
# Secret bytes are never printed or passed as process arguments. Re-running
# atomically rotates the keypair and rolls bex-api; browsers must re-subscribe
# after a public-key change.
set -euo pipefail
cd "$(dirname "$0")/.."

namespace="${BEX_SYSTEM_NAMESPACE:-bex-system}"
secret_name="${BEX_WEBPUSH_SECRET_NAME:-bex-webpush}"
env_file="${BEX_WEBPUSH_ENV_FILE:-.env}"

if [ -f "$env_file" ]; then
  set -a
  # shellcheck disable=SC1091
  source "$env_file"
  set +a
fi

command -v openssl >/dev/null || { echo "error: openssl is required" >&2; exit 1; }
command -v python3 >/dev/null || { echo "error: python3 is required" >&2; exit 1; }

umask 077
secret_dir="$(mktemp -d)"
pem="$secret_dir/vapid.pem"
secret_env="$secret_dir/webpush.env"
cleanup() {
  rm -f "$pem" "$secret_env"
  rmdir "$secret_dir" 2>/dev/null || true
}
trap cleanup EXIT

if [ -z "${BEX_WEBPUSH_VAPID_PUBLIC_KEY:-}" ] || [ -z "${BEX_WEBPUSH_VAPID_PRIVATE_KEY:-}" ]; then
  openssl ecparam -name prime256v1 -genkey -noout -out "$pem"
  mapfile -t keys < <(python3 - "$pem" <<'PY'
import base64, re, subprocess, sys
pem = sys.argv[1]
text = subprocess.check_output(
    ["openssl", "ec", "-in", pem, "-noout", "-text", "-conv_form", "uncompressed"],
    stderr=subprocess.DEVNULL, text=True)
priv_m = re.search(r"priv:\s*((?:[0-9a-fA-F:]+\s*)+)", text)
pub_m = re.search(r"pub:\s*((?:[0-9a-fA-F:]+\s*)+)", text)
if not priv_m or not pub_m:
    sys.exit("openssl did not emit a P-256 key")
def hx(s):
    return bytes.fromhex(re.sub(r"[^0-9a-fA-F]", "", s))
priv, pub = hx(priv_m.group(1)), hx(pub_m.group(1))
if len(priv) == 33 and priv[0] == 0:
    priv = priv[1:]
if len(priv) != 32 or len(pub) != 65 or pub[0] != 4:
    sys.exit("unexpected VAPID key length")
print(base64.urlsafe_b64encode(pub).decode().rstrip("="))
print(base64.urlsafe_b64encode(priv).decode().rstrip("="))
PY
)
  BEX_WEBPUSH_VAPID_PUBLIC_KEY="${keys[0]}"
  BEX_WEBPUSH_VAPID_PRIVATE_KEY="${keys[1]}"
fi

BEX_WEBPUSH_SUBSCRIBER="${BEX_WEBPUSH_SUBSCRIBER:-mailto:webpush@bex.local}"
case "$BEX_WEBPUSH_SUBSCRIBER" in
  mailto:*|https://*) ;;
  *) echo "error: BEX_WEBPUSH_SUBSCRIBER must be a mailto: or https: URI" >&2; exit 1 ;;
esac

if [ "${DRY_RUN:-0}" = "1" ]; then
  echo "would apply Secret $namespace/$secret_name (keys: BEX_WEBPUSH_VAPID_PUBLIC_KEY BEX_WEBPUSH_VAPID_PRIVATE_KEY BEX_WEBPUSH_SUBSCRIBER) and roll bex-api"
  echo "VAPID public key: $BEX_WEBPUSH_VAPID_PUBLIC_KEY"
  exit 0
fi

command -v kubectl >/dev/null || { echo "error: kubectl is required" >&2; exit 1; }

{
  printf 'BEX_WEBPUSH_VAPID_PUBLIC_KEY=%s\n' "$BEX_WEBPUSH_VAPID_PUBLIC_KEY"
  printf 'BEX_WEBPUSH_VAPID_PRIVATE_KEY=%s\n' "$BEX_WEBPUSH_VAPID_PRIVATE_KEY"
  printf 'BEX_WEBPUSH_SUBSCRIBER=%s\n' "$BEX_WEBPUSH_SUBSCRIBER"
} >"$secret_env"

kubectl get namespace "$namespace" >/dev/null 2>&1 || kubectl create namespace "$namespace" >/dev/null
kubectl -n "$namespace" create secret generic "$secret_name" \
  --from-env-file="$secret_env" \
  --dry-run=client -o yaml | kubectl apply -f - >/dev/null

if kubectl -n "$namespace" get deployment/bex-api >/dev/null 2>&1; then
  kubectl -n "$namespace" rollout restart deployment/bex-api >/dev/null
  kubectl -n "$namespace" rollout status deployment/bex-api --timeout=300s >/dev/null
fi

echo "installed $namespace/$secret_name; bex-api rollout is ready"
echo "VAPID public key: $BEX_WEBPUSH_VAPID_PUBLIC_KEY"
