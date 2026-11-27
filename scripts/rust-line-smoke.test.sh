#!/usr/bin/env bash
# Self-test for scripts/rust-line-smoke.sh with a fake docker: it refuses an
# image that is not a digest-pinned rust:1-bookworm, runs the image offline on
# the host's own platform with the source read-only, and passes only when the
# image is the planned Rust release, rustup's default is that release, and
# cargo builds a crate declaring it as its rust-version.
set -euo pipefail

here="$(cd "$(dirname "$0")" && pwd)"
SCRIPT="$here/rust-line-smoke.sh"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
fails=0

IMAGE="rust:1-bookworm@sha256:dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd"
mkdir -p "$tmp/bin"
# The fake answers `docker run` as the image would: FAKE_RUST is its release,
# FAKE_ACTIVE the toolchain rustup reports (its default when unset), and the
# build fails as cargo does when the crate's rust-version is newer, unless
# FAKE_BUILD=pass isolates the version checks.
cat >"$tmp/bin/docker" <<'FAKE'
#!/usr/bin/env bash
printf '%s\n' "$*" >>"$FAKE_LOG"
[ "$1" = run ] || exit "${FAKE_PULL_RC:-0}"
platform="" src=""
while [ $# -gt 0 ]; do
  [ "$1" = --platform ] && platform="$2"
  [ "$1" = -v ] && src="${2%%:*}"
  shift
done
case "$platform" in
  linux/amd64) triple=x86_64-unknown-linux-gnu ;;
  linux/arm64) triple=aarch64-unknown-linux-gnu ;;
esac
echo "rustc ${FAKE_RUST} (0ef1bc386 2026-11-09)"
echo "${FAKE_ACTIVE:-${FAKE_RUST}-${triple} (default)}"
wanted="$(sed -n 's/^rust-version = "\(.*\)"$/\1/p' "$src/Cargo.toml")"
if [ "${FAKE_BUILD:-}" != pass ] && [ "$wanted" != "$FAKE_RUST" ] && [ "$(printf '%s\n%s\n' "$wanted" "$FAKE_RUST" | sort -V | tail -n1)" = "$wanted" ]; then
  echo "error: rustc ${FAKE_RUST} is not supported by the following package:"
  echo "  smoke@0.1.0 requires rustc ${wanted}"
  exit 101
fi
[ -f "$src/src/main.rs" ] || exit 101
echo "smoke ok"
FAKE
chmod +x "$tmp/bin/docker"

expect() {
  local label="$1" want="$2" release="$3"
  shift 3
  : >"$tmp/log"
  set +e
  env PATH="$tmp/bin:$PATH" FAKE_LOG="$tmp/log" "$@" bash "$SCRIPT" "$IMAGE" "$release" >"$tmp/out" 2>&1
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

expect "the planned release on the host platform passes" 0 1.101.0 FAKE_RUST=1.101.0
if ! grep -q -- '--network none' "$tmp/log" || ! grep -qE -- '--platform linux/(amd64|arm64) ' "$tmp/log" \
  || ! grep -qE -- '-v [^ ]+:/src:ro ' "$tmp/log"; then
  echo "FAIL: the smoke run must be offline on the host's own platform with the source read-only: $(cat "$tmp/log")" >&2
  fails=$((fails + 1))
else
  echo "ok: the smoke run is offline on the host's own platform with the source read-only"
fi
expect "a crate newer than the image's release fails" 1 1.101.9 FAKE_RUST=1.101.0
expect "an image carrying another release fails even when its build passes" 1 1.101.0 FAKE_RUST=1.101.1
expect "a toolchain that is not rustup's default fails" 1 1.101.0 FAKE_RUST=1.101.0 \
  FAKE_ACTIVE="1.101.0-x86_64-unknown-linux-gnu (overridden by environment variable RUSTUP_TOOLCHAIN)"
expect "another architecture's toolchain fails" 1 1.101.0 FAKE_RUST=1.101.0 \
  FAKE_ACTIVE="1.101.0-riscv64gc-unknown-linux-gnu (default)"
expect "a release matched only by a regex wildcard fails" 1 1.101.0 FAKE_RUST=1x101x0 FAKE_BUILD=pass
expect "an image that cannot be pulled fails" 1 1.101.0 FAKE_RUST=1.101.0 FAKE_PULL_RC=1 RUST_SMOKE_RETRY_PAUSE=0

refuse() {
  local label="$1"
  shift
  set +e
  env PATH="$tmp/bin:$PATH" FAKE_LOG="$tmp/log" FAKE_RUST=1.101.0 bash "$SCRIPT" "$@" >"$tmp/out" 2>&1
  local got=$?
  set -e
  if [ "$got" -ne 1 ]; then
    echo "FAIL: $label — exit $got, want 1" >&2
    fails=$((fails + 1))
  else
    echo "ok: $label"
  fi
}
refuse "a tag without a digest is refused" "rust:1-bookworm" 1.101.0
refuse "another repository is refused" "${IMAGE/rust:/evil:}" 1.101.0
refuse "another tag of rust is refused" "${IMAGE/1-bookworm/1-alpine}" 1.101.0
refuse "a release that is not 1.minor.patch is refused" "$IMAGE" 1.101
refuse "a Rust 2 release is refused" "$IMAGE" 2.0.0

if [ "$fails" -ne 0 ]; then
  echo "FAIL: $fails rust-line-smoke checks" >&2
  exit 1
fi
echo "PASS: rust-line-smoke.sh"
