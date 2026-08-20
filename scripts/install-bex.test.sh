#!/usr/bin/env bash
# Offline tests for the bex CLI distribution scripts (w4/m35):
# scripts/install-bex.sh against a local fixture release layout, and
# scripts/bex-cli-formula.sh against a fixture checksums.txt. No network.
set -euo pipefail

here="$(cd "$(dirname "$0")" && pwd)"
INSTALL="$here/install-bex.sh"
FORMULA="$here/bex-cli-formula.sh"

fails=0
fail() { echo "FAIL: $*" >&2; fails=$((fails + 1)); }
tmp="$(mktemp -d)"
server_pid=""
# Every command here must be failure-proof: set -e applies inside the trap,
# and kill/wait status (143 from the killed server) must not become the
# script's exit code.
cleanup() {
  if [ -n "$server_pid" ]; then
    kill "$server_pid" 2>/dev/null || true
    wait "$server_pid" 2>/dev/null || true
  fi
  rm -rf "$tmp"
}
trap cleanup EXIT

os="$(uname -s | tr '[:upper:]' '[:lower:]')"
case "$(uname -m)" in
  arm64 | aarch64) arch="arm64" ;;
  *) arch="amd64" ;;
esac
version="0.1.0"
artifact="bex-${version}-${os}-${arch}"

# ── fixture release layout ──────────────────────────────────────────────────
srv="$tmp/srv"
release_dir="$srv/dl/bex-cli/v${version}" # %2F in the URL decodes to this path
mkdir -p "$release_dir" "$srv/repos/bex-co/bex" "$tmp/pkg/$artifact"
printf '#!/bin/sh\necho "bex v%s (fixture)"\n' "$version" > "$tmp/pkg/$artifact/bex"
chmod +x "$tmp/pkg/$artifact/bex"
tar -C "$tmp/pkg" -czf "$release_dir/$artifact.tar.gz" "$artifact"
if command -v sha256sum >/dev/null; then
  (cd "$release_dir" && sha256sum "$artifact.tar.gz" > checksums.txt)
else
  (cd "$release_dir" && shasum -a 256 "$artifact.tar.gz" > checksums.txt)
fi
# Offline fixture: a placeholder bundle plus a PATH stub for cosign so the
# installer exercises the verify-blob gate without network TUF (codex round-16 #1).
printf '{}\n' > "$release_dir/checksums.txt.sigstore.json"
mkdir -p "$tmp/bin"
cat > "$tmp/bin/cosign" <<'EOF'
#!/bin/sh
# Fixture stub: accept verify-blob when a bundle path was supplied.
for a in "$@"; do
  case "$a" in
    --bundle) exit 0 ;;
  esac
done
echo "cosign stub: unexpected argv: $*" >&2
exit 1
EOF
chmod +x "$tmp/bin/cosign"
export PATH="$tmp/bin:$PATH"
# The API fixture lists a non-CLI release first to prove tag-prefix filtering.
cat > "$srv/repos/bex-co/bex/releases" <<EOF
[
  {"tag_name": "operator/v9.9.9", "draft": false},
  {"tag_name": "bex-cli/v${version}", "draft": false}
]
EOF

python3 - "$srv" > "$tmp/port" <<'PY' &
import http.server, os, sys
os.chdir(sys.argv[1])
class Quiet(http.server.SimpleHTTPRequestHandler):
    def log_message(self, *args):
        pass
httpd = http.server.ThreadingHTTPServer(("127.0.0.1", 0), Quiet)
print(httpd.server_address[1], flush=True)
httpd.serve_forever()
PY
server_pid=$!
for _ in $(seq 1 50); do [ -s "$tmp/port" ] && break; sleep 0.1; done
port="$(cat "$tmp/port")"
[ -n "$port" ] || { echo "FAIL: fixture server did not start" >&2; exit 1; }
base="http://127.0.0.1:$port"

# ── installer: latest-version resolution via the API fixture ────────────────
out="$(BEX_API_URL="$base" BEX_DOWNLOAD_URL="$base/dl" BEX_INSTALL_DIR="$tmp/bin1" sh "$INSTALL" 2>&1)" ||
  fail "installer (latest) exited nonzero:\n$out"
[ -x "$tmp/bin1/bex" ] || fail "installer (latest) did not install bex"
"$tmp/bin1/bex" | grep -q "bex v$version (fixture)" || fail "installed binary is not the fixture"
echo "$out" | grep -q "v$version" || fail "installer output missing resolved version:\n$out"

# ── installer: pinned version skips the API entirely ────────────────────────
mv "$srv/repos" "$tmp/repos-hidden"
out="$(BEX_VERSION="$version" BEX_API_URL="$base" BEX_DOWNLOAD_URL="$base/dl" BEX_INSTALL_DIR="$tmp/bin2" sh "$INSTALL" 2>&1)" ||
  fail "installer (pinned) exited nonzero:\n$out"
[ -x "$tmp/bin2/bex" ] || fail "installer (pinned) did not install bex"
mv "$tmp/repos-hidden" "$srv/repos"

# ── installer: a version with no release asset fails with a clear error ─────
if out="$(BEX_VERSION="9.9.9" BEX_API_URL="$base" BEX_DOWNLOAD_URL="$base/dl" BEX_INSTALL_DIR="$tmp/bin-missing" sh "$INSTALL" 2>&1)"; then
  fail "installer accepted a nonexistent version:\n$out"
fi
echo "$out" | grep -q "download failed" || fail "missing-asset error not reported:\n$out"
[ ! -e "$tmp/bin-missing/bex" ] || fail "binary installed despite missing asset"

# ── installer: no bex-cli release in the API listing fails clearly ──────────
printf '[{"tag_name": "operator/v9.9.9", "draft": false}]\n' > "$srv/repos/bex-co/bex/releases"
if out="$(BEX_API_URL="$base" BEX_DOWNLOAD_URL="$base/dl" BEX_INSTALL_DIR="$tmp/bin-none" sh "$INSTALL" 2>&1)"; then
  fail "installer resolved a version with no bex-cli release:\n$out"
fi
echo "$out" | grep -q "no bex-cli/v\* release found" || fail "no-release error not reported:\n$out"
cat > "$srv/repos/bex-co/bex/releases" <<EOF
[
  {"tag_name": "operator/v9.9.9", "draft": false},
  {"tag_name": "bex-cli/v${version}", "draft": false}
]
EOF

# ── installer: checksums.txt without this platform's entry aborts pre-download
cp "$release_dir/checksums.txt" "$tmp/checksums.orig"
printf '%s  bex-9.9.9-other-arch.tar.gz\n' "0000000000000000000000000000000000000000000000000000000000000000" > "$release_dir/checksums.txt"
if out="$(BEX_VERSION="$version" BEX_API_URL="$base" BEX_DOWNLOAD_URL="$base/dl" BEX_INSTALL_DIR="$tmp/bin-noentry" sh "$INSTALL" 2>&1)"; then
  fail "installer accepted checksums.txt without the platform entry:\n$out"
fi
echo "$out" | grep -q "no entry for" || fail "missing-entry error not reported:\n$out"
cp "$tmp/checksums.orig" "$release_dir/checksums.txt"

# ── installer: missing Sigstore bundle must abort before install ────────────
rm -f "$release_dir/checksums.txt.sigstore.json"
if out="$(BEX_VERSION="$version" BEX_API_URL="$base" BEX_DOWNLOAD_URL="$base/dl" BEX_INSTALL_DIR="$tmp/bin-nosig" sh "$INSTALL" 2>&1)"; then
  fail "installer accepted a release without checksums.txt.sigstore.json:\n$out"
fi
echo "$out" | grep -q "sigstore.json" || fail "missing-signature error not reported:\n$out"
[ ! -e "$tmp/bin-nosig/bex" ] || fail "binary installed despite missing signature"
printf '{}\n' > "$release_dir/checksums.txt.sigstore.json"

# ── installer: checksum mismatch must abort before installing ───────────────
awk '{ replacement = substr($1, 1, 1) == "0" ? "1" : "0"; print replacement substr($1, 2) "  " $2 }' \
  "$release_dir/checksums.txt" > "$release_dir/checksums.bad" &&
  mv "$release_dir/checksums.bad" "$release_dir/checksums.txt"
if out="$(BEX_VERSION="$version" BEX_API_URL="$base" BEX_DOWNLOAD_URL="$base/dl" BEX_INSTALL_DIR="$tmp/bin3" sh "$INSTALL" 2>&1)"; then
  fail "installer accepted a corrupted checksum:\n$out"
fi
echo "$out" | grep -q "checksum mismatch" || fail "mismatch error not reported:\n$out"
[ ! -e "$tmp/bin3/bex" ] || fail "binary installed despite checksum mismatch"

# ── formula renderer: four platforms from one checksums.txt ─────────────────
cat > "$tmp/checksums.txt" <<EOF
1111111111111111111111111111111111111111111111111111111111111111  bex-0.1.0-linux-amd64.tar.gz
2222222222222222222222222222222222222222222222222222222222222222  bex-0.1.0-linux-arm64.tar.gz
3333333333333333333333333333333333333333333333333333333333333333  bex-0.1.0-darwin-amd64.tar.gz
4444444444444444444444444444444444444444444444444444444444444444  bex-0.1.0-darwin-arm64.tar.gz
EOF
formula="$(VERSION=0.1.0 bash "$FORMULA" "$tmp/checksums.txt")" || fail "formula render exited nonzero"
for want in 'version "0.1.0"' 'bex-cli%2Fv0.1.0' \
  1111111111111111111111111111111111111111111111111111111111111111 \
  4444444444444444444444444444444444444444444444444444444444444444 \
  'class Bex < Formula' 'bin.install'; do
  echo "$formula" | grep -qF "$want" || fail "formula missing: $want"
done

# ── formula renderer: a missing platform entry must abort ───────────────────
grep -v darwin-arm64 "$tmp/checksums.txt" > "$tmp/checksums.short"
if VERSION=0.1.0 bash "$FORMULA" "$tmp/checksums.short" >/dev/null 2>&1; then
  fail "formula render accepted an incomplete checksums.txt"
fi

if [ "$fails" -gt 0 ]; then
  echo "$fails test(s) failed" >&2
  exit 1
fi
echo "PASS install-bex + bex-cli-formula"
