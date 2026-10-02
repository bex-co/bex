#!/usr/bin/env bash
# Run the actual audit in a temporary repository without .env or cluster access.
# Optional script path supports reproducing the pre-fix false pass.
set -euo pipefail
cd "$(dirname "$0")/.."
python3 - "${1:-scripts/delete-audit.sh}" <<'PY'
import json
import os
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile

source = Path(sys.argv[1]).resolve()
with tempfile.TemporaryDirectory(prefix="bex-delete-audit-") as directory:
    root = Path(directory)
    (root / "scripts").mkdir()
    (root / "bin").mkdir()
    audit = root / "scripts/delete-audit.sh"
    shutil.copyfile(source, audit)
    kubectl = root / "bin/kubectl"
    kubectl.write_text('''#!/usr/bin/env python3
import json, os, sys
args = sys.argv[1:]
with open(os.environ["AUDIT_CALLS"], "a") as calls:
    calls.write(json.dumps(args) + "\\n")
assert args[0] == "get", "audit attempted a mutation"
assert args[args.index("-n") + 1] == "fixture-workspace", "wrong namespace"
kind = args[1]
output = args[args.index("-o") + 1]
if kind in ("secret", "certificates.cert-manager.io"):
    assert output == "name", "audit requested Secret or Certificate contents"
    assert "-l" not in args, "TLS lookup must use an exact name"
fixture = os.environ["AUDIT_FIXTURE"]
name = "red-residue-kv-tls"
if name in args and kind in ("secret", "certificates.cert-manager.io"):
    if fixture == "secret-error" and kind == "secret" or fixture == "certificate-error" and kind == "certificates.cert-manager.io":
        print("Forbidden: fixture denies metadata read", file=sys.stderr)
        sys.exit(1)
    if fixture == "secret-only" and kind == "secret" or fixture == "certificate-only" and kind == "certificates.cert-manager.io":
        print(kind + "/" + name)
        sys.exit(0)
# Other-namespace and similarly-prefixed objects exist in the decoy fixture,
# but cannot match these exact reads. A broad read would return the residue.
if fixture == "decoys" and kind in ("secret", "certificates.cert-manager.io") and name not in args and "red-residue" not in args and "red-residue-auth" not in args:
    print(kind + "/red-residue-kv-tls-other")
elif output == "jsonpath={.items}":
    print("[]")
''')
    kubectl.chmod(0o755)
    failures = []
    for fixture, should_pass in [
        ("secret-only", False), ("certificate-only", False),
        ("clean", True), ("secret-error", False),
        ("certificate-error", False), ("decoys", True),
    ]:
        calls = root / (fixture + ".calls")
        # A fresh environment excludes backup credentials and all .env input.
        env = {"PATH": str(root / "bin") + os.pathsep + os.environ["PATH"],
               "APPS_NS": "fixture-workspace", "AUDIT_FIXTURE": fixture,
               "AUDIT_CALLS": str(calls)}
        result = subprocess.run(["bash", str(audit), "--kv", "red-residue"],
                                env=env, text=True, capture_output=True)
        output = result.stdout + result.stderr
        requests = [json.loads(line) for line in calls.read_text().splitlines()]
        tls_kinds = {args[1] for args in requests if "red-residue-kv-tls" in args}
        valid = (result.returncode == 0) == should_pass
        valid &= tls_kinds == {"secret", "certificates.cert-manager.io"}
        # Validate recorded requests outside the mock too: the audit suppresses
        # kubectl stderr, so a mock assertion alone could hide a bad query.
        valid &= all(args[0] == "get" and "-n" in args
                     and args[args.index("-n") + 1] == "fixture-workspace"
                     for args in requests)
        valid &= all(args[args.index("-o") + 1] == "name" and "-l" not in args
                     for args in requests
                     if args[1] in ("secret", "certificates.cert-manager.io"))
        valid &= "SKIP  KeyValue S3 check" in output
        valid &= "Traceback" not in output
        if not valid:
            failures.append(fixture)
            print(f"FAIL {fixture}: exit={result.returncode}, TLS reads={sorted(tls_kinds)}")
            print(output)
        else:
            print(f"PASS {fixture}: exit={result.returncode}, exact TLS metadata checks; S3 explicitly skipped")
    if failures:
        sys.exit(1)
PY
