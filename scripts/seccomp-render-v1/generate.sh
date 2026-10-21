#!/usr/bin/env bash
# Regenerates deploy/gitops/base/seccomp/bex-render-v1.json (w6/m150): runs the
# generator natively on linux/amd64 (production nodes) and linux/arm64 (local
# CAPD and CI), because containerd's profile is architecture-specific, and
# merges both into one profile. runc skips syscall names unknown to the node's
# architecture, so the union is safe on either. Needs Docker (emulation for
# the foreign architecture).
set -euo pipefail
here=$(cd "$(dirname "$0")" && pwd)
root=$(cd "$here/../.." && pwd)
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
for arch in amd64 arm64; do
  (cd "$here" && GOWORK=off CGO_ENABLED=0 GOOS=linux GOARCH=$arch go build -o "$tmp/gen-$arch" .)
  docker run --rm --platform "linux/$arch" -v "$tmp:/g:ro" \
    alpine:3.21@sha256:48b0309ca019d89d40f670aa1bc06e426dc0931948452e8491e3d65087abc07d "/g/gen-$arch" >"$tmp/$arch.json"
done
python3 - "$tmp/amd64.json" "$tmp/arm64.json" >"$root/deploy/gitops/base/seccomp/bex-render-v1.json" <<'PY'
import json, sys
profiles = [json.load(open(p)) for p in sys.argv[1:]]
out = {"defaultAction": profiles[0]["defaultAction"], "defaultErrnoRet": profiles[0].get("defaultErrnoRet"),
       "architectures": [], "syscalls": []}
for p in profiles:
    for a in p.get("architectures") or []:
        if a not in out["architectures"]:
            out["architectures"].append(a)
# Merge rules that share action, args and errno; names are unioned and sorted.
merged = {}
for p in profiles:
    for r in p["syscalls"]:
        key = json.dumps({k: r.get(k) for k in ("action", "args", "errnoRet", "includes", "excludes")}, sort_keys=True)
        names = merged.setdefault(key, (r, set()))[1]
        names.update(r["names"])
for key, (rule, names) in merged.items():
    rule = {k: v for k, v in rule.items() if v not in (None, [], {})}
    rule["names"] = sorted(names)
    out["syscalls"].append(rule)
out = {k: v for k, v in out.items() if v is not None}
json.dump(out, sys.stdout, indent=2, sort_keys=True)
sys.stdout.write("\n")
PY
echo "wrote deploy/gitops/base/seccomp/bex-render-v1.json" >&2
