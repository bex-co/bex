#!/usr/bin/env bash
# Self-test for scripts/render-examples-corpus.sh (w33/001): the weekly corpus
# guard must see every render-examples Blueprint, so only a 404 may mean "no
# file here". Any other gh failure (a rate limit, a 5xx) must stop the fetch
# instead of quietly shrinking the corpus the guard validates.
set -euo pipefail
here="$(cd "$(dirname "$0")" && pwd)"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
fails=0

# A fake gh: three live repositories and one archived. alpha has render.yaml,
# beta only render.yml, gamma neither. FAIL_PATH names the one contents read
# that answers 403 instead.
mkdir -p "$tmp/bin"
cat >"$tmp/bin/gh" <<'GH'
#!/usr/bin/env bash
for arg; do
  case "$arg" in orgs/* | repos/*) url="$arg" ;; esac
done
case "$url" in
  orgs/render-examples/repos*)
    printf '%s\n' alpha beta gamma
    exit 0
    ;;
esac
if [ "$url" = "${FAIL_PATH:-}" ]; then
  echo 'gh: API rate limit exceeded (HTTP 403)' >&2
  exit 1
fi
case "$url" in
  repos/render-examples/alpha/contents/render.yaml) printf 'services: []  # alpha\n' | base64 ;;
  repos/render-examples/beta/contents/render.yml) printf 'services: []  # beta\n' | base64 ;;
  *)
    echo 'gh: Not Found (HTTP 404)' >&2
    exit 1
    ;;
esac
GH
chmod +x "$tmp/bin/gh"

# check <label> <want-rc> <needle> [FAIL_PATH]
check() {
  local label="$1" want="$2" needle="$3" fail="${4:-}" out got dir
  dir="$(mktemp -d "$tmp/out.XXXXXX")"
  set +e
  out="$(PATH="$tmp/bin:$PATH" FAIL_PATH="$fail" bash "$here/render-examples-corpus.sh" "$dir" 2>&1)"
  got=$?
  set -e
  if [ "$got" -ne "$want" ] || ! printf '%s' "$out" | grep -qF "$needle"; then
    echo "FAIL: $label — exit $got (want $want): $out" >&2
    fails=$((fails + 1))
  else
    echo "ok: $label"
  fi
  last="$dir"
}

check "a 404 skips the path; render.yml is the fallback" 0 "fetched 2 render.yaml files"
if ! grep -qF '# alpha' "$last/alpha.yaml" || ! grep -qF '# beta' "$last/beta.yaml" || [ -e "$last/gamma.yaml" ]; then
  echo "FAIL: the fetched files are not alpha's render.yaml and beta's render.yml" >&2
  fails=$((fails + 1))
fi
check "a failure other than 404 stops the fetch" 1 "beta/render.yaml: gh: API rate limit exceeded" \
  repos/render-examples/beta/contents/render.yaml

[ "$fails" -eq 0 ] || exit 1
