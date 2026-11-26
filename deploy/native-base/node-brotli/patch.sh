#!/bin/sh
# Rebuild only an existing reviewed Node executable with its line's Brotli
# security fix: Google's matching release for the EOL lines, Node's own
# in-tree cherry-pick for Render's dated Node 24 defaults. npm and the
# selected Node/ABI versions stay unchanged.
set -eu

common_mode=0
expected_version=
source_checksum=
if [ "$#" -ne 0 ]; then
  [ "$#" -eq 3 ] && [ "$1" = --common-bookworm ]
  printf '%s\n' "$2" | grep -Eq '^[0-9]+(\.[0-9]+){2}$'
  printf '%s\n' "$3" | grep -Eq '^[a-f0-9]{64}$'
  common_mode=1
  expected_version=$2
  source_checksum=$3
fi
[ -x /usr/local/bin/node ] || exit 0
node_version=$(/usr/local/bin/node -p 'process.versions.node')
[ "$common_mode" -eq 0 ] || [ "$node_version" = "$expected_version" ]
brotli_patch=
brotli_source=source
case "$node_version" in
  14.21.2)
    node_sha=d8f09a0f16773a77613c3817606f6d455624992d9c43443aca15e91807a1ff03
    brotli_version=1.0.10
    brotli_sha=b517ebdb5266b963d28d6e639b973479874ec72086deb6ee4a2d639e1b13606b
    ;;
  14.21.3)
    node_sha=458ec092e60ad700ddcf079cb63d435c15da4c7bb3d3f99b9a8e58a99e54075e
    brotli_version=1.0.10
    brotli_sha=b517ebdb5266b963d28d6e639b973479874ec72086deb6ee4a2d639e1b13606b
    ;;
  16.20.2)
    node_sha=576f1a03c455e491a8d132b587eb6b3b84651fc8974bb3638433dd44d22c8f49
    brotli_version=1.0.10
    brotli_sha=b517ebdb5266b963d28d6e639b973479874ec72086deb6ee4a2d639e1b13606b
    ;;
  18.20.8)
    node_sha=36a7bf1a76d62ce4badd881ee5974a323c70e1d8d19165732684e145632460d9
    brotli_version=1.1.1
    brotli_sha=1a84dfaa740994d19254bf78aa7d74a4a8040d1638f3d5f2cdecb188fd520da8
    ;;
  20.20.2)
    node_sha=7aeeacdb858299e09a3e0510d4bb8b266923894a9e3ac0058ba89d4ecf4a4cca
    brotli_version=1.1.1
    brotli_sha=1a84dfaa740994d19254bf78aa7d74a4a8040d1638f3d5f2cdecb188fd520da8
    ;;
  14.*|16.*|18.*|20.*)
    [ "$common_mode" -eq 1 ] || { echo "Unreviewed EOL Node source: $node_version" >&2; exit 1; }
    node_sha=$source_checksum
    # Early Node18/20 also bundled Brotli1.0. Keep the selected binary's
    # actual dependency family so its GYP source list remains compatible.
    case "$(/usr/local/bin/node -p 'process.versions.brotli')" in
      1.0.*)
        brotli_version=1.0.10
        brotli_sha=b517ebdb5266b963d28d6e639b973479874ec72086deb6ee4a2d639e1b13606b ;;
      1.1.*)
        brotli_version=1.1.1
        brotli_sha=1a84dfaa740994d19254bf78aa7d74a4a8040d1638f3d5f2cdecb188fd520da8 ;;
      *) echo 'unexpected EOL Node Brotli dependency family' >&2; exit 1 ;;
    esac
    ;;
  # Render's dated defaults (w3/m672). Both bundle Node's Brotli 1.2.0 tree
  # before the fix; Node's own cherry-pick makes deps/brotli/c identical to
  # 24.22.1's, which still reports 1.2.0.
  24.14.1|24.21.0)
    [ "$common_mode" -eq 0 ] || { echo "Node 24 backports are published donors, never tenant builds" >&2; exit 1; }
    case "$node_version" in
      24.14.1) node_sha=7822507713f202cf2a551899d250259643f477b671706db421a6fb55c4aa0991 ;;
      24.21.0) node_sha=a6f54defb6fd7c84f41dba13d61e78e9b4e0961712cf61f29715c05f5ced94fc ;;
    esac
    brotli_version=1.2.0
    brotli_patch=node-24-brotli-49dff6f57aa7.patch
    brotli_source="patch nodejs/node@49dff6f57aa7"
    brotli_sha=1e9ee182923958541a120582078b087380ab38e6850153cdc3cdec33cf331876
    ;;
  24.*)
    # 24.22.1 is the line's first release carrying Node's fix (w1/135).
    minor=${node_version#24.}
    patch_level=${minor#*.}
    minor=${minor%%.*}
    if [ "$minor" -gt 22 ] || { [ "$minor" -eq 22 ] && [ "$patch_level" -ge 1 ]; }; then
      exit 0
    fi
    echo "Unreviewed Node 24 source without the Brotli fix: $node_version" >&2
    exit 1
    ;;
  *) exit 0 ;;
esac
[ "$common_mode" -eq 0 ] || [ "$node_sha" = "$source_checksum" ]

. /etc/os-release
# Node 24 requires GCC 12.2, which Bookworm has and Bullseye lacks; the EOL
# lines compile on the Bullseye companion baseline unless a common build asks.
expected_suite=bookworm
if [ "$common_mode" -eq 0 ] && [ -z "$brotli_patch" ]; then
  expected_suite=bullseye
fi
if [ "$VERSION_CODENAME" != "$expected_suite" ]; then
  echo "Node $node_version backport must compile on Debian $expected_suite" >&2
  exit 1
fi

script_dir=$(cd "$(dirname "$0")" && pwd)
work_dir=$(mktemp -d)
trap 'rm -rf "$work_dir"' EXIT HUP INT TERM
cd "$work_dir"

# HTTPS checksums are recorded from the official Node SHASUMS256.txt and
# Google Brotli release archives; see README.md beside this script. Node's own
# fix is vendored beside the script because generated diffs are not stable.
curl --fail --location --retry 3 "https://nodejs.org/dist/v$node_version/node-v$node_version.tar.xz" -o node.tar.xz
printf '%s  node.tar.xz\n' "$node_sha" | sha256sum -c -
if [ -n "$brotli_patch" ]; then
  cp "$script_dir/$brotli_patch" brotli.patch
  printf '%s  brotli.patch\n' "$brotli_sha" | sha256sum -c -
else
  curl --fail --location --retry 3 "https://codeload.github.com/google/brotli/tar.gz/refs/tags/v$brotli_version" -o brotli.tar.gz
  printf '%s  brotli.tar.gz\n' "$brotli_sha" | sha256sum -c -
  tar -xzf brotli.tar.gz
fi
tar -xJf node.tar.xz

/usr/local/bin/node -p 'JSON.stringify(process.versions)' > original-versions.json
original_npm=$(npm --version)
cd "node-v$node_version"
if [ -n "$brotli_patch" ]; then
  patch -p1 --forward --no-backup-if-mismatch < ../brotli.patch
  # The repaired decoder is byte-for-byte Node 24.22.1's.
  sha256sum -c - <<'SUMS'
d48bc54c60553238d50db2fcd606522efd447d33a37d6a2d5c2ea1ece22067c3  deps/brotli/c/dec/decode.c
e2f5fdc3ae41e6b7e8986538e421ae79c095541bacf32f2bbfccf8e056405444  deps/brotli/c/include/brotli/decode.h
SUMS
else
  # The matching 1.0/1.1 security release changes the decoder, not the Node
  # ABI. Keep Node's GYP metadata; replace only Google's vendored C sources.
  rm -rf deps/brotli/c
  cp -a "../brotli-$brotli_version/c" deps/brotli/c
fi
# Match Node's official companion fix: expose the new decoder error constant.
python3 - <<'PY'
from pathlib import Path
p = Path('src/node_zlib.cc')
s = p.read_text()
anchor = '  NODE_DEFINE_CONSTANT(target, BROTLI_DECODER_ERROR_FORMAT_DISTANCE);'
if s.count(anchor) != 1:
    raise SystemExit('Node Brotli error constant anchor changed')
p.write_text(s.replace(anchor, anchor + '\n  NODE_DEFINE_CONSTANT(target, BROTLI_DECODER_ERROR_FORMAT_BLOCK_SWITCH);'))
PY
if [ "$common_mode" -eq 1 ]; then
  # Node14 and early Node16 reject Bookworm's system Python3.11. The
  # compatible interpreter exists only in the isolated build stage.
  /usr/local/bin/python3.9 -c 'import sys; assert sys.version_info[:2] == (3, 9)'
  /usr/local/bin/python3.9 ./configure
else
  ./configure
fi
# Node/V8 compilation is memory hungry. Bound parallelism independently of
# the host's (potentially large) CPU count; tenant builds never run this.
if ! make -s -j2 node > "$work_dir/build.log" 2>&1; then
  tail -n 100 "$work_dir/build.log" >&2
  exit 1
fi

./out/Release/node - "$work_dir/original-versions.json" "$brotli_version" <<'JS'
const assert = require('assert');
const before = require(process.argv[2]);
for (const key of ['node', 'modules', 'napi', 'v8', 'uv', 'openssl', 'zlib', 'icu']) {
  assert.strictEqual(process.versions[key], before[key], `${key} version changed`);
}
assert.strictEqual(process.versions.brotli, process.argv[3]);
JS
install -m 755 out/Release/node /usr/local/bin/node
[ "$(npm --version)" = "$original_npm" ]
mkdir -p /usr/share/bex-node-brotli
printf 'Node %s; source sha256:%s; Brotli %s; %s sha256:%s\n' \
  "$node_version" "$node_sha" "$brotli_version" "$brotli_source" "$brotli_sha" \
  > /usr/share/bex-node-brotli/sources.txt
printf 'Compiler baseline: Debian %s; %s\n' "$VERSION_CODENAME" "$(getconf GNU_LIBC_VERSION)" \
  >> /usr/share/bex-node-brotli/sources.txt
