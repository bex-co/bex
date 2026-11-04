#!/usr/bin/env bash
# Runs the sandbox pool's real gVisor bootstrap items (install, containerd
# runtime config, gate) from infra/clusterapi/overlays/hetzner-caph/
# sandbox-pool.yaml under /bin/sh, the way cloud-init's runcmd script does,
# followed by a stand-in for the `kubeadm join` item CAPI appends. A node must
# never reach the join without gVisor installed and verified (w9/074).
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
manifest="$repo_root/infra/clusterapi/overlays/hetzner-caph/sandbox-pool.yaml"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
fails=0
fail() {
  echo "FAIL: $*" >&2
  fails=$((fails + 1))
}

# The gVisor items: from the install through the gate.
python3 - "$manifest" >"$tmp/items.sh" <<'PY'
import sys, yaml
docs = [d for d in yaml.safe_load_all(open(sys.argv[1])) if d]
spec = next(d for d in docs if d["kind"] == "KubeadmConfigTemplate")["spec"]["template"]["spec"]
cmds = spec["preKubeadmCommands"]
start = next(i for i, c in enumerate(cmds) if "GVURL=" in c)
end = next(i for i, c in enumerate(cmds) if "gVisor bootstrap gate failed" in c)
print("\n".join(cmds[start : end + 1]))
PY
grep -q 'GVURL=' "$tmp/items.sh" || fail "could not extract the gVisor install item"
grep -q 'gVisor bootstrap gate failed' "$tmp/items.sh" || fail "the gVisor gate item is missing"

sha512_of() {
  if command -v sha512sum >/dev/null; then sha512sum "$1" | awk '{print $1}'; else shasum -a 512 "$1" | awk '{print $1}'; fi
}

# run_case NAME ARCH WGET_MODE TAMPER -> sets $status, $out_dir
run_case() {
  local name="$1" arch="$2" wget_mode="$3" tamper="$4"
  local root="$tmp/$name"
  mkdir -p "$root/bin" "$root/etc" "$root/tmp" "$root/stub" "$root/release"
  printf 'fake runsc\n' >"$root/release/runsc"
  printf 'fake shim\n' >"$root/release/containerd-shim-runsc-v1"
  : >"$root/etc/config.toml"
  local runsc_sha shim_sha
  runsc_sha="$(sha512_of "$root/release/runsc")"
  shim_sha="$(sha512_of "$root/release/containerd-shim-runsc-v1")"
  [ "$tamper" = yes ] && printf 'tampered\n' >"$root/release/runsc"

  cat >"$root/stub/wget" <<EOF
#!/bin/sh
echo attempt >>"$root/wget.calls"
[ "$wget_mode" = fail ] && exit 4
for url in "\$@"; do case "\$url" in http*) cp "$root/release/\${url##*/}" . ;; esac; done
EOF
  printf '#!/bin/sh\necho %s\n' "$arch" >"$root/stub/uname"
  printf '#!/bin/sh\nexit 0\n' >"$root/stub/sleep"
  if ! command -v sha512sum >/dev/null; then
    printf '#!/bin/sh\nexec shasum -a 512 "$@"\n' >"$root/stub/sha512sum"
  fi
  chmod +x "$root"/stub/*

  {
    sed -e "s#RUNSC_SHA512=[0-9a-f]*#RUNSC_SHA512=$runsc_sha#" \
      -e "s#SHIM_SHA512=[0-9a-f]*#SHIM_SHA512=$shim_sha#" \
      -e "s#/usr/local/bin#$root/bin#g" -e "s#/etc/containerd#$root/etc#g" \
      -e "s#cd /tmp#cd $root/tmp#g" "$tmp/items.sh"
    echo "echo JOINED"
  } >"$root/runcmd.sh"

  set +e
  PATH="$root/stub:$PATH" /bin/sh "$root/runcmd.sh" >"$root/out" 2>&1
  status=$?
  set -e
  out_dir="$root"
}

run_case good x86_64 ok no
[ "$status" -eq 0 ] && grep -q JOINED "$out_dir/out" || fail "a verified install must reach the join (status $status)"
[ -x "$out_dir/bin/runsc" ] && [ -x "$out_dir/bin/containerd-shim-runsc-v1" ] || fail "binaries were not installed"

run_case download_fails x86_64 fail no
[ "$status" -ne 0 ] && ! grep -q JOINED "$out_dir/out" || fail "a failed download must stop before the join"
[ "$(wc -l <"$out_dir/wget.calls" | tr -d ' ')" -eq 5 ] || fail "the download must be retried 5 times"
grep -q "gVisor bootstrap gate failed" "$out_dir/out" || fail "the gate must explain why it stopped"

run_case digest_mismatch x86_64 ok yes
[ "$status" -ne 0 ] && ! grep -q JOINED "$out_dir/out" || fail "a digest mismatch must stop before the join"
[ ! -e "$out_dir/bin/runsc" ] || fail "a tampered runsc must not be installed"

run_case wrong_arch aarch64 ok no
[ "$status" -ne 0 ] && ! grep -q JOINED "$out_dir/out" || fail "a non-x86_64 node must stop before the join"

if [ "$fails" -ne 0 ]; then
  echo "capi-sandbox-bootstrap: $fails failure(s)" >&2
  exit 1
fi
echo "PASS capi-sandbox-bootstrap"
