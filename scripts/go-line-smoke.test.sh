#!/usr/bin/env bash
# Self-test for scripts/go-line-smoke.sh with a fake docker: it refuses an
# image that is not a digest-pinned golang line, runs the image offline on the
# host's own platform, and passes only when the image is the planned Go
# release, keeps GOTOOLCHAIN=local and builds the module.
set -euo pipefail

here="$(cd "$(dirname "$0")" && pwd)"
SCRIPT="$here/go-line-smoke.sh"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
fails=0

IMAGE="golang:1.28-bookworm@sha256:dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd"
mkdir -p "$tmp/bin"
# The fake answers `docker run` as the image would: FAKE_GO is its release,
# FAKE_TOOLCHAIN its GOTOOLCHAIN, FAKE_BUILD=fail breaks the build.
cat >"$tmp/bin/docker" <<'FAKE'
#!/usr/bin/env bash
printf '%s\n' "$*" >>"$FAKE_LOG"
[ "$1" = run ] || exit "${FAKE_PULL_RC:-0}"
platform=""
while [ $# -gt 0 ]; do
  [ "$1" = --platform ] && platform="$2"
  shift
done
echo "go version go${FAKE_GO} ${platform}"
echo "${FAKE_TOOLCHAIN:-local}"
if [ "${FAKE_BUILD:-}" = fail ]; then
  echo "go: go.mod requires go >= 1.28.0 (running go ${FAKE_GO}; GOTOOLCHAIN=local)"
  exit 1
fi
echo "smoke ok"
FAKE
chmod +x "$tmp/bin/docker"

run() {
  : >"$tmp/log"
  env PATH="$tmp/bin:$PATH" FAKE_LOG="$tmp/log" "$@" bash "$SCRIPT" "$IMAGE" 1.28.0
}

expect() {
  local label="$1" want="$2"
  shift 2
  set +e
  run "$@" >"$tmp/out" 2>&1
  local got=$?
  set -e
  if [ "$got" -ne "$want" ]; then
    echo "FAIL: $label — exit $got, want $want" >&2
    cat "$tmp/out" >&2
    fails=$((fails + 1))
  else
    echo "ok: $label"
  fi
}

expect "the planned release on the host platform passes" 0 FAKE_GO=1.28.0
if ! grep -q -- '--network none' "$tmp/log" || ! grep -qE -- '--platform linux/(amd64|arm64) ' "$tmp/log"; then
  echo "FAIL: the smoke run must be offline on the host's own platform: $(cat "$tmp/log")" >&2
  fails=$((fails + 1))
else
  echo "ok: the smoke run is offline on the host's own platform"
fi
expect "an image carrying another release fails" 1 FAKE_GO=1.27.5 FAKE_BUILD=fail
expect "an image that would download a toolchain fails" 1 FAKE_GO=1.28.0 FAKE_TOOLCHAIN=auto
expect "an image that cannot be pulled fails" 1 FAKE_GO=1.28.0 FAKE_PULL_RC=1 GO_SMOKE_RETRY_PAUSE=0

refuse() {
  local label="$1"
  shift
  set +e
  env PATH="$tmp/bin:$PATH" FAKE_LOG="$tmp/log" FAKE_GO=1.28.0 bash "$SCRIPT" "$@" >"$tmp/out" 2>&1
  local got=$?
  set -e
  if [ "$got" -ne 1 ]; then
    echo "FAIL: $label — exit $got, want 1" >&2
    fails=$((fails + 1))
  else
    echo "ok: $label"
  fi
}
refuse "a tag without a digest is refused" "golang:1.28-bookworm" 1.28.0
refuse "another repository is refused" "${IMAGE/golang/evil}" 1.28.0
refuse "a release off the image's line is refused" "$IMAGE" 1.27.5
refuse "a release that is not major.minor.patch is refused" "$IMAGE" 1.28

if [ "$fails" -ne 0 ]; then
  echo "FAIL: $fails go-line-smoke checks" >&2
  exit 1
fi
echo "PASS: go-line-smoke.sh"
