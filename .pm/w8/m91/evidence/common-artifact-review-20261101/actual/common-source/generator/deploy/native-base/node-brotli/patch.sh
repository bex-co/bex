#!/bin/sh
# Rebuild only the existing EOL Node executable with Google's matching Brotli
# security release. npm and the selected Node/ABI versions stay unchanged.
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
case "$node_version" in
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
  *) exit 0 ;;
esac
[ "$common_mode" -eq 0 ] || [ "$node_sha" = "$source_checksum" ]

. /etc/os-release
expected_suite=bullseye
[ "$common_mode" -eq 0 ] || expected_suite=bookworm
if [ "$VERSION_CODENAME" != "$expected_suite" ]; then
  echo "Node backports must compile on the Bullseye companion baseline" >&2
  exit 1
fi

work_dir=$(mktemp -d)
trap 'rm -rf "$work_dir"' EXIT HUP INT TERM
cd "$work_dir"

# HTTPS checksums are recorded from the official Node SHASUMS256.txt and
# Google Brotli release archives; see README.md beside this script.
curl --fail --location --retry 3 "https://nodejs.org/dist/v$node_version/node-v$node_version.tar.xz" -o node.tar.xz
printf '%s  node.tar.xz\n' "$node_sha" | sha256sum -c -
curl --fail --location --retry 3 "https://codeload.github.com/google/brotli/tar.gz/refs/tags/v$brotli_version" -o brotli.tar.gz
printf '%s  brotli.tar.gz\n' "$brotli_sha" | sha256sum -c -
tar -xJf node.tar.xz
tar -xzf brotli.tar.gz

/usr/local/bin/node -p 'JSON.stringify(process.versions)' > original-versions.json
original_npm=$(npm --version)
cd "node-v$node_version"
# The matching 1.0/1.1 security release changes the decoder, not the Node ABI.
# Keep Node's GYP metadata; replace only Google's vendored C sources.
rm -rf deps/brotli/c
cp -a "../brotli-$brotli_version/c" deps/brotli/c
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
printf 'Node %s; source sha256:%s; Brotli %s; source sha256:%s\n' \
  "$node_version" "$node_sha" "$brotli_version" "$brotli_sha" \
  > /usr/share/bex-node-brotli/sources.txt
printf 'Compiler baseline: Debian %s; %s\n' "$VERSION_CODENAME" "$(getconf GNU_LIBC_VERSION)" \
  >> /usr/share/bex-node-brotli/sources.txt
