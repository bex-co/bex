#!/usr/bin/env bash
# Custody contract of scripts/lib/analytics-bootstrap.sh (w7/m153): an unowned
# Secret is applied and made adoptable, a SealedSecret-owned Secret is never
# rewritten, CA drift under sealed custody fails closed, and SEAL_TO reseals.
set -euo pipefail
cd "$(dirname "$0")/.."

tmp="$(mktemp -d)"
trap 'rm -rf -- "$tmp"' EXIT
mkdir -p "$tmp/bin"

cat >"$tmp/bin/kubectl" <<'MOCK'
#!/usr/bin/env bash
set -euo pipefail
printf '%s\n' "$*" >>"$T/trace"
b64() { printf '%s' "$1" | base64 | tr -d '\n'; }
case "$*" in
  *"get clusters.postgresql.cnpg.io bex-db"*) printf 'bex-db-1' ;;
  *"get secret bex-db-ca"*) b64 "$T_DB_CA" ;;
  *"get secret grafana-product-analytics"*ownerReferences*) printf '%s' "${T_OWNER:-}" ;;
  *"get secret grafana-product-analytics"*data.password*) [ -z "${T_PASSWORD:-}" ] || b64 "$T_PASSWORD" ;;
  *"get secret grafana-product-analytics"*ca*) b64 "$T_SECRET_CA" ;;
  *"exec -i bex-db-1 -c postgres -- psql"*) cat >>"$T/psql" ;;
  "apply -f -") cat >>"$T/applied" ;;
  *"annotate --overwrite secret grafana-product-analytics sealedsecrets.bitnami.com/managed=true") ;;
  *) echo "unexpected kubectl invocation: $*" >&2; exit 1 ;;
esac
MOCK
cat >"$tmp/bin/kubeseal" <<'MOCK'
#!/usr/bin/env bash
set -euo pipefail
printf '%s\n' "$*" >>"$T/trace"
{ echo "# sealed"; cat; } | tee "$T/sealed-input" >/dev/null
echo "kind: SealedSecret"
MOCK
chmod +x "$tmp/bin/kubectl" "$tmp/bin/kubeseal"

pw_old="$(printf 'a%.0s' {1..64})"
b64() { printf '%s' "$1" | base64 | tr -d '\n'; }
fail=0
check() { if eval "$2"; then echo "ok: $1"; else echo "FAIL: $1" >&2; fail=1; fi; }

# run NAME [ENV=VALUE...] — fresh state dir, returns the script's exit code in $rc.
run() {
  local name="$1"
  shift
  T="$tmp/$name"
  mkdir -p "$T"
  : >"$T/trace" >"$T/psql" >"$T/applied"
  rc=0
  env PATH="$tmp/bin:$PATH" T="$T" T_DB_CA=ca-current "$@" \
    bash scripts/lib/analytics-bootstrap.sh product >"$T/out" 2>"$T/err" || rc=$?
}

run fresh
check "fresh install succeeds" '[ "$rc" = 0 ]'
check "fresh install applies the Secret with the current CA" 'grep -qF "ca.crt: $(b64 ca-current)" "$T/applied"'
check "fresh install marks the Secret adoptable" 'grep -q "annotate --overwrite secret grafana-product-analytics sealedsecrets.bitnami.com/managed=true" "$T/trace"'
minted="$(sed -n "s/^\\\\set analytics_password '\\(.*\\)'$/\\1/p" "$T/psql")"
check "fresh install sets the role to the minted password" '[[ "$minted" =~ ^[a-f0-9]{64}$ ]] && grep -qF "password: $(b64 "$minted")" "$T/applied"'

run unowned T_PASSWORD="$pw_old" T_SECRET_CA=ca-current
check "unowned Secret keeps its password and is re-applied" '[ "$rc" = 0 ] && grep -qF "password: $(b64 "$pw_old")" "$T/applied"'
check "unowned Secret is marked adoptable" 'grep -q "sealedsecrets.bitnami.com/managed=true" "$T/trace"'

run sealed T_OWNER=grafana-product-analytics T_PASSWORD="$pw_old" T_SECRET_CA=ca-current
check "sealed Secret succeeds" '[ "$rc" = 0 ]'
check "sealed Secret is never rewritten or re-annotated" '[ ! -s "$T/applied" ] && ! grep -q annotate "$T/trace"'
check "role password follows the sealed Secret" 'grep -qF "\\set analytics_password '"'"'$pw_old'"'"'" "$T/psql"'

run drift T_OWNER=grafana-product-analytics T_PASSWORD="$pw_old" T_SECRET_CA=ca-stale
check "sealed CA drift fails closed" '[ "$rc" != 0 ] && grep -q "SEAL_TO" "$T/err"'
check "sealed CA drift touches neither database nor Secret" '[ ! -s "$T/psql" ] && [ ! -s "$T/applied" ]'

run reseal T_OWNER=grafana-product-analytics T_PASSWORD="$pw_old" T_SECRET_CA=ca-stale SEAL_TO="$tmp/reseal/out.yaml"
check "SEAL_TO reseals the current CA with the kept password" '[ "$rc" = 0 ] && grep -qF "ca.crt: $(b64 ca-current)" "$T/sealed-input" && grep -qF "password: $(b64 "$pw_old")" "$T/sealed-input"'
check "SEAL_TO writes the manifest and leaves the owned Secret alone" 'grep -q "kind: SealedSecret" "$tmp/reseal/out.yaml" && [ ! -s "$T/applied" ]'

for name in fresh unowned sealed drift reseal; do
  check "$name: no credential in argv or output" '! grep -qF "$pw_old" "$tmp/$name/trace" "$tmp/$name/out" "$tmp/$name/err"'
done

exit "$fail"
