#!/usr/bin/env bash
# Exercises the actual audit with metadata fixtures, without a cluster or .env.
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
audit="${1:-$repo_root/scripts/delete-audit.sh}"
tmp="$(mktemp -d)"
trap 'rm -rf -- "$tmp"' EXIT
mkdir -p "$tmp/repo/scripts" "$tmp/bin"
# Preserve the script's bootstrap while keeping the real repository's .env out
# of the test. The optional path permits the same cases against a before-fix copy.
cp "$audit" "$tmp/repo/scripts/delete-audit.sh"

cat >"$tmp/bin/kubectl" <<'PY'
#!/usr/bin/env python3
import json
import os
import sys

def contract_error(error_type, error, traceback):
    with open(os.environ["AUDIT_TEST_TRACE"], "a") as trace:
        trace.write("HARNESS_ERROR: " + str(error) + "\n")

sys.excepthook = contract_error
args = sys.argv[1:]
with open(os.environ["AUDIT_TEST_TRACE"], "a") as trace:
    trace.write(json.dumps(args) + "\n")
assert args[0] == "get", "the audit must never mutate cluster resources"
kind = args[1]
namespace = output = selector = name = None
i = 2
while i < len(args):
    flag = args[i]
    if flag in ("-n", "-o", "-l"):
        value = args[i + 1]
        if flag == "-n":
            namespace = value
        elif flag == "-o":
            output = value
        else:
            selector = value
        i += 2
    elif flag == "--ignore-not-found":
        i += 1
    else:
        assert not flag.startswith("-"), "unexpected or unsafe kubectl flag"
        assert name is None, "multiple names are outside this fixture's scope"
        name = flag
        i += 1
assert namespace == "tea-audit", "the audit must use the supplied namespace"
assert kind in {
    "keyvalues.app.bex.co", "pvc", "statefulset", "secret",
    "certificates.cert-manager.io", "certificaterequests.cert-manager.io",
    "cronjobs", "jobs", "pods", "databases.app.bex.co", "cluster.postgresql.cnpg.io",
    "apps.app.bex.co", "secrets", "serviceaccounts", "networkpolicies",
    "images.kpack.io", "builds.kpack.io", "certificate",
}, "unexpected resource kind"
if kind in ("secret", "secrets"):
    assert output == "name", "Secret reads must be name-only"
    if os.environ["AUDIT_TEST_TARGET"] == "--kv":
        assert name is not None, "Key Value Secret reads must be exact"

scenario = os.environ["AUDIT_TEST_SCENARIO"]
kv = "red-audit"
certificate = kv + "-kv-tls"
if ((scenario == "secret-error" and kind == "secret" and name == certificate)
    or (scenario == "certificate-error" and kind == "certificates.cert-manager.io")
    or (scenario == "request-error" and kind == "certificaterequests.cert-manager.io")
    or (scenario == "backup-error" and kind == "jobs")
    or (scenario.endswith("helper-error") and kind == "jobs")
    or (scenario == "keyvalue-error" and kind == "keyvalues.app.bex.co")):
    print("Forbidden: simulated metadata access refusal", file=sys.stderr)
    sys.exit(43)
if scenario == "request-invalid" and kind == "certificaterequests.cert-manager.io":
    print("{invalid metadata")
    sys.exit(0)

def resource(resource_kind, resource_name, **metadata):
    return {"kind": resource_kind, "metadata": {
        "name": resource_name, "namespace": "tea-audit", **metadata,
    }}

resources = []
if scenario == "tls-only":
    # cert-manager's observed issued-Secret shape: issuance annotations but no
    # ownerReferences. No labels or owner refs may be required to detect it.
    secret = resource("secret", certificate,
        uid="2dab3f40-8ab4-4f56-b0ac-087a4ecb99fb",
        annotations={"cert-manager.io/certificate-name": certificate,
                     "cert-manager.io/issuer-name": "letsencrypt-prod",
                     "cert-manager.io/alt-names": kv + ".kv.example.test"})
    secret["type"] = "kubernetes.io/tls"
    secret["data"] = {"tls.key": "SENSITIVE_FIXTURE_MUST_NOT_BE_READ"}
    resources.append(secret)
elif scenario == "certificate-only":
    resources.append(resource("certificates.cert-manager.io", certificate))
elif scenario == "request-annotation":
    resources.append(resource("certificaterequests.cert-manager.io", "unpredictable-request-name",
        annotations={"cert-manager.io/certificate-name": certificate}))
elif scenario == "request-owner":
    resources.append(resource("certificaterequests.cert-manager.io", certificate + "-17",
        ownerReferences=[{"apiVersion": "cert-manager.io/v1", "kind": "Certificate",
                          "name": certificate, "uid": "owned-certificate-uid"}]))
elif scenario == "backup-orphan":
    resources.append(resource("jobs", kv + "-backup-123",
        labels={"app.bex.co/keyvalue": kv, "app.bex.co/component": "keyvalue-backup"}))
elif scenario == "app-helper-orphan":
    resources.append(resource("jobs", kv + "-build",
        labels={"app.bex.co/app": kv}))
elif scenario == "db-helper-orphan":
    resources.append(resource("jobs", kv + "-purge",
        labels={"app.bex.co/database": kv, "app.bex.co/component": "database-backup-purge"}))
elif scenario == "neighbors":
    # Similar names, another namespace and a same-name non-Certificate owner
    # do not establish a relation to this exact issued certificate.
    for resource_kind in ("secret", "certificates.cert-manager.io"):
        resources.append(resource(resource_kind, certificate + "-other"))
        other_namespace = resource(resource_kind, certificate)
        other_namespace["metadata"]["namespace"] = "tea-other"
        resources.append(other_namespace)
    resources.append(resource("certificaterequests.cert-manager.io", certificate + "-other-1",
        annotations={"cert-manager.io/certificate-name": certificate + "-other"},
        ownerReferences=[{"apiVersion": "cert-manager.io/v1", "kind": "Certificate",
                          "name": certificate + "-other"}]))
    resources.append(resource("certificaterequests.cert-manager.io", certificate + "-unrelated",
        ownerReferences=[{"apiVersion": "app.bex.co/v1alpha1", "kind": "KeyValue",
                          "name": certificate}]))

items = [item for item in resources if item["kind"] == kind
         and item["metadata"]["namespace"] == namespace
         and (name is None or item["metadata"]["name"] == name)]
if selector:
    labels = dict(part.split("=", 1) for part in selector.split(","))
    items = [item for item in items if all(
        item["metadata"].get("labels", {}).get(key) == value
        for key, value in labels.items())]
if output == "name":
    for item in items:
        print(kind + "/" + item["metadata"]["name"])
elif output == "jsonpath={.items}" and kind != "secret":
    # The original audit used this format; support it for the mutation check.
    print(json.dumps(items))
elif kind == "certificaterequests.cert-manager.io":
    assert output == 'jsonpath={range .items[*]}{.metadata}{"\\n"}{end}', "request output must be metadata-only"
    for item in items:
        print(json.dumps(item["metadata"]))
else:
    raise AssertionError("unexpected output format")
PY

cat >"$tmp/bin/aws" <<'SH'
#!/usr/bin/env bash
set -euo pipefail
if [[ "$*" != 's3 ls s3://audit-backups/red-audit/ --recursive --endpoint-url https://s3.example.test' ]]; then
  echo 'HARNESS_ERROR: unexpected S3 operation' >>"$AUDIT_TEST_TRACE"
  exit 91
fi
case "$AUDIT_TEST_SCENARIO" in
  s3-error) exit 44 ;;
  s3-orphan) echo '2026-10-02 00:00:00 100 dump.rdb' ;;
esac
SH
chmod +x "$tmp/bin/kubectl" "$tmp/bin/aws"

run_case() {
  local scenario="$1" expected_status="$2" expected_text="$3" s3="${4:-0}"
  local target="${5:---kv}" destination="" endpoint="" status=0
  if [ "$s3" = "1" ]; then
    destination="s3://audit-backups"
    endpoint="https://s3.example.test"
  fi
  : >"$tmp/trace"
  env -i PATH="$tmp/bin:$PATH" APPS_NS=tea-audit \
    AUDIT_TEST_SCENARIO="$scenario" AUDIT_TEST_TRACE="$tmp/trace" AUDIT_TEST_TARGET="$target" \
    BEX_KV_BACKUP_DESTINATION="$destination" BEX_KV_BACKUP_ENDPOINT="$endpoint" \
    bash "$tmp/repo/scripts/delete-audit.sh" "$target" red-audit \
    >"$tmp/output" 2>&1 || status=$?
  local case_failed=0
  if { [ "$expected_status" = pass ] && [ "$status" -ne 0 ]; } || \
     { [ "$expected_status" = fail ] && [ "$status" -eq 0 ]; }; then
    echo "FAIL: $scenario expected $expected_status, got exit $status" >&2
    case_failed=1
  fi
  if [ -n "$expected_text" ] && ! grep -Fq -- "$expected_text" "$tmp/output"; then
    echo "FAIL: $scenario did not report $expected_text" >&2
    case_failed=1
  fi
  if grep -Eq 'SENSITIVE_FIXTURE|Traceback|AssertionError|HARNESS_ERROR' "$tmp/output" "$tmp/trace"; then
    echo "FAIL: $scenario exposed material or violated the command contract" >&2
    case_failed=1
  fi
  # Validate the recorded requests independently: kubectl stderr is suppressed,
  # so an assertion inside the command shim cannot be the only safety guard.
  if ! python3 - "$tmp/trace" "$target" <<'VERIFY'
import json, sys
requests = [json.loads(line) for line in open(sys.argv[1]) if not line.startswith("HARNESS_ERROR:")]
assert requests, "audit made no metadata queries"
for args in requests:
    assert args[0] == "get", "audit attempted a mutation"
    assert "-n" in args and args[args.index("-n") + 1] == "tea-audit", "wrong namespace"
    kind = args[1]
    output = args[args.index("-o") + 1]
    if kind in ("secret", "secrets"):
        assert output == "name", "audit requested Secret contents"
        if sys.argv[2] == "--kv":
            assert "-l" not in args and any(name in args for name in
                ("red-audit", "red-audit-auth", "red-audit-kv-tls")), "Secret query was not exact"
    if kind == "certificates.cert-manager.io":
        assert output == "name" and "-l" not in args and "red-audit-kv-tls" in args, "Certificate query was not exact"
    if kind == "certificaterequests.cert-manager.io":
        assert output == 'jsonpath={range .items[*]}{.metadata}{"\\n"}{end}', "request query included non-metadata fields"
VERIFY
  then
    case_failed=1
  fi
  if [ "$case_failed" -ne 0 ]; then
    cat "$tmp/output" >&2
    failures=$((failures + 1))
    return
  fi
  echo "PASS: $scenario"
}

failures=0
run_case absent pass 'SKIP  KeyValue S3 check'
run_case tls-only fail 'KeyValue TLS secret red-audit-kv-tls still exists'
run_case certificate-only fail 'KeyValue TLS certificates.cert-manager.io red-audit-kv-tls still exists'
run_case request-annotation fail '1 CertificateRequest(s) still exist'
run_case request-owner fail '1 CertificateRequest(s) still exist'
run_case neighbors pass '0 failed'
run_case secret-error fail 'could not verify KeyValue TLS secret'
run_case certificate-error fail 'could not verify KeyValue TLS certificates.cert-manager.io'
run_case request-error fail 'cannot inspect CertificateRequests'
run_case request-invalid fail 'cannot inspect CertificateRequests'
run_case backup-orphan fail '1 keyvalue-backup jobs remain'
run_case backup-error fail 'Cannot inspect jobs'
run_case keyvalue-error fail ''
run_case s3-absent pass 'no KeyValue backup objects' 1
run_case s3-orphan fail '1 KeyValue backup object(s) remain' 1
run_case s3-error fail '' 1
run_case app-absent pass 'no App-owned jobs' 0 --app
run_case app-helper-orphan fail '1 App-owned jobs' 0 --app
run_case app-helper-error fail 'Cannot inspect jobs' 0 --app
run_case static-helper-error fail 'Cannot inspect jobs' 0 --static
run_case db-absent pass 'no durable backup-purge jobs' 0 --db
run_case db-helper-orphan fail '1 backup-purge jobs remain' 0 --db
run_case db-helper-error fail 'Cannot inspect jobs' 0 --db
[ "$failures" -eq 0 ] || exit 1
echo 'PASS: delete audit detects TLS residue, scopes metadata reads, and fails closed'
