#!/usr/bin/env bash
# Pure-bash regression tests for scripts/packer-plugin-install.sh (w1/m66 F12).
# Network- and Packer-free: `curl` and `packer` are shell-function fakes on PATH,
# so the test asserts the ORDER that matters — the artifact is verified against
# the repository-pinned checksum BEFORE `packer plugins install` is ever reached,
# and a mismatch installs nothing at all.
set -euo pipefail
cd "$(dirname "$0")/.."

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

fails=0
assert() {
  local description="$1"
  shift
  if "$@"; then
    echo "    ok: $description"
  else
    echo "FAIL: $description" >&2
    fails=$((fails + 1))
  fi
}

# A fake plugin "release": a zip whose single entry is the binary name the
# installer expects, plus its real sha256 so the happy path is genuine.
version="9.9.9"
os=$(uname -s | tr '[:upper:]' '[:lower:]')
case "$(uname -m)" in
  x86_64 | amd64) arch=amd64 ;;
  arm64 | aarch64) arch=arm64 ;;
  *)
    echo "unsupported arch for this test: $(uname -m)" >&2
    exit 1
    ;;
esac
binary="packer-plugin-hcloud_v${version}_x5.0_${os}_${arch}"
artifact="${binary}.zip"

mkdir -p "$tmp/release"
printf '#!/bin/sh\necho fake plugin\n' > "$tmp/release/$binary"
chmod +x "$tmp/release/$binary"
(cd "$tmp/release" && zip -q "$tmp/$artifact" "$binary")

if command -v sha256sum >/dev/null 2>&1; then
  real_sum=$(sha256sum "$tmp/$artifact" | awk '{print $1}')
else
  real_sum=$(shasum -a 256 "$tmp/$artifact" | awk '{print $1}')
fi

# Fakes on PATH: curl serves the local zip; packer records that it was called.
mkdir -p "$tmp/bin"
cat > "$tmp/bin/curl" <<EOF
#!/usr/bin/env bash
# usage mirrors the script: curl -fsSLo <dest> <url>
dest=""
while [ \$# -gt 0 ]; do
  case "\$1" in
    -fsSLo) dest="\$2"; shift 2 ;;
    *) shift ;;
  esac
done
cp "$tmp/$artifact" "\$dest"
EOF
cat > "$tmp/bin/packer" <<EOF
#!/usr/bin/env bash
echo "packer \$*" >> "$tmp/packer-calls"
EOF
chmod +x "$tmp/bin/curl" "$tmp/bin/packer"

# A scratch copy of the repo files the script reads, so the real pin file and
# template are never touched.
run_installer() {
  local pinned_sum="$1" template_version="$2"
  rm -f "$tmp/packer-calls"
  mkdir -p "$tmp/repo/infra/packer" "$tmp/repo/scripts"
  cp scripts/packer-plugin-install.sh "$tmp/repo/scripts/"
  printf '# test fixture\n%s  %s\n' "$pinned_sum" "$artifact" > "$tmp/repo/infra/packer/plugin-checksums.txt"
  cat > "$tmp/repo/infra/packer/bex-worker.pkr.hcl" <<EOF
packer {
  required_plugins {
    hcloud = {
      source  = "github.com/hetznercloud/hcloud"
      version = "= ${template_version}"
    }
  }
}
EOF
  PATH="$tmp/bin:$PATH" bash "$tmp/repo/scripts/packer-plugin-install.sh" "$version" > "$tmp/out" 2>&1
}

installed() { grep -q "plugins install" "$tmp/packer-calls" 2>/dev/null; }
not_installed() { ! installed; }

echo "packer-plugin-install.sh"

# 1. Matching checksum + exactly-pinned template => installs.
if run_installer "$real_sum" "$version"; then
  assert "a verified artifact is installed" installed
else
  echo "FAIL: happy path exited non-zero:" >&2
  cat "$tmp/out" >&2
  fails=$((fails + 1))
fi

# 2. Mismatched checksum => fails closed, installs nothing.
bad_sum="0000000000000000000000000000000000000000000000000000000000000000"
if run_installer "$bad_sum" "$version"; then
  echo "FAIL: a checksum mismatch must exit non-zero" >&2
  fails=$((fails + 1))
else
  assert "a mismatched artifact is never installed" not_installed
  assert "the mismatch is reported" grep -q "checksum mismatch" "$tmp/out"
fi

# 3. No pinned entry for the artifact => fails closed before downloading.
if run_installer "$real_sum" "$version" && sed -i.bak "s/$artifact/some-other-artifact.zip/" "$tmp/repo/infra/packer/plugin-checksums.txt"; then
  rm -f "$tmp/packer-calls"
  if PATH="$tmp/bin:$PATH" bash "$tmp/repo/scripts/packer-plugin-install.sh" "$version" > "$tmp/out" 2>&1; then
    echo "FAIL: an unpinned artifact must exit non-zero" >&2
    fails=$((fails + 1))
  else
    assert "an unpinned artifact is never installed" not_installed
    assert "the missing pin is reported" grep -q "no pinned checksum" "$tmp/out"
  fi
fi

# 4. Template that does not pin the exact version => refuse (an open constraint
#    would let packer build load something other than what was verified).
cat > "$tmp/repo/infra/packer/bex-worker.pkr.hcl" <<EOF
packer {
  required_plugins {
    hcloud = {
      source  = "github.com/hetznercloud/hcloud"
      version = ">= 1.6.0"
    }
  }
}
EOF
printf '%s  %s\n' "$real_sum" "$artifact" > "$tmp/repo/infra/packer/plugin-checksums.txt"
rm -f "$tmp/packer-calls"
if PATH="$tmp/bin:$PATH" bash "$tmp/repo/scripts/packer-plugin-install.sh" "$version" > "$tmp/out" 2>&1; then
  echo "FAIL: an open version constraint must exit non-zero" >&2
  fails=$((fails + 1))
else
  assert "an open version constraint is refused" not_installed
fi

# 5. The REAL repository state must be self-consistent: the template's pinned
#    version has a checksum entry for the CI platform (linux/amd64).
real_version=$(sed -n 's/^[[:space:]]*version[[:space:]]*=[[:space:]]*"=[[:space:]]*\([0-9.]*\)".*/\1/p' infra/packer/bex-worker.pkr.hcl | head -1)
assert "the template pins an exact plugin version" test -n "$real_version"
assert "the pin file covers linux/amd64 for the pinned version" \
  grep -q "packer-plugin-hcloud_v${real_version}_x5.0_linux_amd64.zip" infra/packer/plugin-checksums.txt

if [ "$fails" -ne 0 ]; then
  echo "$fails check(s) failed" >&2
  exit 1
fi
echo "all packer-plugin-install checks passed"
