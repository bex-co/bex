#!/usr/bin/env bash
# Runs scripts/pty-signal-echo-probe.py under the runsc the sandbox pool
# installs, so a gVisor change that stops a sandbox terminal echoing ^C, ^\ and
# ^Z like Linux (w4/m545), or flushing the line on ^C and erasing it on ^U
# (w4/m574), fails here before it reaches the pool.
#
# The pool's own install item is extracted from
# infra/clusterapi/overlays/hetzner-caph/sandbox-pool.yaml and run with its
# destination redirected, so the download URL and the pinned SHA-512 digests
# are exactly the pool's. The probe runs natively first: a native mismatch is
# the probe's fault, not gVisor's.
#
# Needs Linux x86_64, wget, python3 with PyYAML, and sudo for `runsc do` (CI:
# .github/workflows/sandbox-runsc-pty.yml on ubuntu-latest).
#
#   bash scripts/sandbox-runsc-pty-check.sh [sandbox-pool.yaml]
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
manifest="${1:-$repo_root/infra/clusterapi/overlays/hetzner-caph/sandbox-pool.yaml}"
probe="$repo_root/scripts/pty-signal-echo-probe.py"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
mkdir -p "$tmp/bin" "$tmp/download"

python3 - "$manifest" >"$tmp/install.sh" <<'PY'
import sys, yaml
docs = [d for d in yaml.safe_load_all(open(sys.argv[1])) if d]
spec = next(d for d in docs if d["kind"] == "KubeadmConfigTemplate")["spec"]["template"]["spec"]
print(next(c for c in spec["preKubeadmCommands"] if "GVURL=" in c))
PY
sed -i.bak -e "s#cd /tmp#cd $tmp/download#" -e "s#/usr/local/bin/#$tmp/bin/#g" "$tmp/install.sh"
grep -q "$tmp/bin/" "$tmp/install.sh" || { echo "FAIL: the pool's gVisor install item no longer copies into /usr/local/bin/" >&2; exit 1; }

echo "==> the sandbox pool's gVisor install item"
if ! sh "$tmp/install.sh"; then
  echo "FAIL: the pool's install item did not download and verify runsc" >&2
  exit 1
fi
"$tmp/bin/runsc" --version

echo "==> native $(uname -sr)"
python3 -I "$probe"

echo "==> the pool's runsc"
sudo "$tmp/bin/runsc" --network=none "do" python3 -I "$probe"
