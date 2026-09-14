#!/usr/bin/env bash
# Exercise download recovery and authentication without a cluster or registry.
set -euo pipefail

repo_root=$(cd "$(dirname "$0")/.." && pwd)
tmp=$(mktemp -d)
trap 'rm -rf -- "$tmp"' EXIT
mkdir -p "$tmp/scripts" "$tmp/deploy" "$tmp/bin"
cp "$repo_root/scripts/helm-artifact.sh" "$tmp/scripts/"
printf 'reviewed chart\n' > "$tmp/reviewed.tgz"
digest=$(sha256sum "$tmp/reviewed.tgz" | awk '{print $1}')
printf 'test-chart|1.0.0|https://charts.example.test|%s|-\n' "$digest" > "$tmp/deploy/helm-artifacts.lock"

cat > "$tmp/bin/helm" <<'SH'
#!/usr/bin/env bash
set -euo pipefail
attempt=0
[ ! -f "$FIXTURE/attempt" ] || attempt=$(cat "$FIXTURE/attempt")
attempt=$((attempt + 1))
printf '%s\n' "$attempt" > "$FIXTURE/attempt"
destination=${!#}
if [ "$MODE" = permanent ] || { [ "$MODE" = transient ] && [ "$attempt" -lt 3 ]; }; then
  # A failed download must never make a partial archive usable.
  printf 'partial' > "$destination/test-chart-1.0.0.tgz"
  echo 'Error: Get chart: EOF' >&2
  exit 1
fi
cp "$FIXTURE/reviewed.tgz" "$destination/test-chart-1.0.0.tgz"
if [ "$MODE" = corrupt ]; then
  printf 'tampered' >> "$destination/test-chart-1.0.0.tgz"
fi
echo 'Helm download diagnostic'
SH
printf '#!/usr/bin/env bash\nexit 0\n' > "$tmp/bin/sleep"
chmod +x "$tmp/bin/helm" "$tmp/bin/sleep"
export PATH="$tmp/bin:$PATH" FIXTURE="$tmp"

for mode in success transient permanent corrupt; do
  export MODE="$mode"
  rm -f "$tmp/attempt"
  output=''
  if output=$(bash "$tmp/scripts/helm-artifact.sh" pull test-chart "$tmp/$mode" 2> "$tmp/error"); then
    case "$mode" in permanent|corrupt) echo "FAIL: accepted $mode download" >&2; exit 1 ;; esac
    [ "$output" = "$tmp/$mode/test-chart-1.0.0.tgz" ]
    cmp "$output" "$tmp/reviewed.tgz"
  else
    case "$mode" in success|transient) cat "$tmp/error" >&2; exit 1 ;; esac
    [ -z "$output" ]
  fi
  case "$mode" in
    transient|permanent) [ "$(cat "$tmp/attempt")" = 3 ] ;;
    *) [ "$(cat "$tmp/attempt")" = 1 ] ;;
  esac
  echo "PASS: $mode"
done
