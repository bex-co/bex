#!/usr/bin/env bash
# Exercise download recovery, authentication and mirror reuse without a
# cluster or registry.
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

# mirror: a chart already published at its reviewed digest is not downloaded
# from upstream again; a missing or different digest still is.
oci='sha256:1111111111111111111111111111111111111111111111111111111111111111'
printf 'test-chart|1.0.0|https://charts.example.test|%s|%s\n' "$digest" "$oci" > "$tmp/deploy/helm-artifacts.lock"
cat > "$tmp/bin/oras" <<'SH'
#!/usr/bin/env bash
[ "$1 $2" = "manifest fetch" ] && [[ " $* " == *" --descriptor "* ]] || exit 1
[ -n "$MIRRORED_DIGEST" ] || { echo 'not found' >&2; exit 1; }
printf '{"digest":"%s"}\n' "$MIRRORED_DIGEST"
SH
cat > "$tmp/bin/helm" <<'SH'
#!/usr/bin/env bash
set -euo pipefail
destination=${!#}
case "$2" in
  oci://*)
    cp "$FIXTURE/reviewed.tgz" "$destination/test-chart-1.0.0.tgz"
    echo "Digest: $OCI_DIGEST"
    ;;
  *)
    echo upstream >> "$FIXTURE/upstream"
    echo 'Error: 504 Gateway Timeout' >&2
    exit 1
    ;;
esac
SH
chmod +x "$tmp/bin/oras" "$tmp/bin/helm"
export OCI_DIGEST="$oci"

rm -f "$tmp/upstream"
MIRRORED_DIGEST="$oci" bash "$tmp/scripts/helm-artifact.sh" mirror "$tmp/reuse" > "$tmp/out" 2> "$tmp/error" \
  || { cat "$tmp/error" >&2; echo "FAIL: mirror of an already-published chart" >&2; exit 1; }
[ ! -f "$tmp/upstream" ] || { echo "FAIL: re-downloaded an already-mirrored chart" >&2; exit 1; }
grep -q "Already mirrored: .*test-chart:1.0.0@$oci" "$tmp/out"
echo "PASS: mirror reuses the published digest"

for mirrored in '' 'sha256:2222222222222222222222222222222222222222222222222222222222222222'; do
  rm -f "$tmp/upstream"
  if MIRRORED_DIGEST="$mirrored" bash "$tmp/scripts/helm-artifact.sh" mirror "$tmp/stale" > /dev/null 2>&1; then
    echo "FAIL: mirror succeeded with upstream down and no reviewed copy (${mirrored:-missing})" >&2; exit 1
  fi
  [ "$(wc -l < "$tmp/upstream" | tr -d " ")" = 3 ] || { echo "FAIL: did not fetch upstream for ${mirrored:-missing}" >&2; exit 1; }
done
echo "PASS: mirror fetches upstream when the published digest is missing or differs"
