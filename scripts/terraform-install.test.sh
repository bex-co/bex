#!/usr/bin/env bash
# Offline tests for the rootless Terraform installer. The release download and
# platform probe are faked; ZIP creation/extraction and SHA-256 verification are
# real, including the failure paths that must install nothing.
set -euo pipefail
cd "$(dirname "$0")/.."

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
mkdir -p "$tmp/bin" "$tmp/release"

version=9.9.9
artifact="$tmp/release/terraform_${version}_linux_arm64.zip"
printf '#!/usr/bin/env bash\necho "Terraform v9.9.9"\n' >"$tmp/release/terraform"
chmod +x "$tmp/release/terraform"
python3 - "$tmp/release/terraform" "$artifact" <<'PY'
import sys
from zipfile import ZIP_DEFLATED, ZipFile

source, artifact = sys.argv[1:]
with ZipFile(artifact, "w", compression=ZIP_DEFLATED) as archive:
    archive.write(source, "terraform")
PY

if command -v sha256sum >/dev/null 2>&1; then
  real_sum="$(sha256sum "$artifact" | awk '{print $1}')"
else
  real_sum="$(shasum -a 256 "$artifact" | awk '{print $1}')"
fi

cat >"$tmp/bin/curl" <<'SH'
#!/usr/bin/env bash
set -euo pipefail
destination=""
while [ "$#" -gt 0 ]; do
  case "$1" in
  -fsSLo)
    destination="$2"
    shift 2
    ;;
  *) shift ;;
  esac
done
cp "$FAKE_TERRAFORM_ARCHIVE" "$destination"
SH
cat >"$tmp/bin/uname" <<'SH'
#!/usr/bin/env bash
case "${1:-}" in
-s) printf '%s\n' "${FAKE_UNAME_S:-Linux}" ;;
-m) printf '%s\n' "${FAKE_UNAME_M:-aarch64}" ;;
*) exit 1 ;;
esac
SH
chmod +x "$tmp/bin/curl" "$tmp/bin/uname"

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

run_installer() {
  local runner_temp="$1" checksum="$2"
  mkdir -p "$runner_temp"
  : >"$runner_temp/github-path"
  PATH="$tmp/bin:$PATH" \
    FAKE_TERRAFORM_ARCHIVE="$artifact" \
    RUNNER_TEMP="$runner_temp" \
    GITHUB_PATH="$runner_temp/github-path" \
    bash scripts/terraform-install.sh "$version" "$checksum" \
    >"$runner_temp/output" 2>&1
}

echo "terraform-install.sh"

happy="$tmp/happy"
if run_installer "$happy" "$real_sum"; then
  assert "verified Terraform is installed" test -x "$happy/terraform-${version}/terraform"
  assert "the install directory is exported to GITHUB_PATH" \
    grep -Fxq "$happy/terraform-${version}" "$happy/github-path"
  assert "the installed binary was executed" grep -Fq 'Terraform v9.9.9' "$happy/output"
else
  echo "FAIL: verified install exited non-zero" >&2
  fails=$((fails + 1))
fi

bad_sum=0000000000000000000000000000000000000000000000000000000000000000
mismatch="$tmp/mismatch"
if run_installer "$mismatch" "$bad_sum"; then
  echo "FAIL: checksum mismatch must exit non-zero" >&2
  fails=$((fails + 1))
else
  assert "a mismatched release is not installed" test ! -e "$mismatch/terraform-${version}/terraform"
  assert "the checksum mismatch is reported" grep -Fq 'checksum mismatch' "$mismatch/output"
fi

unsupported="$tmp/unsupported"
mkdir -p "$unsupported"
: >"$unsupported/github-path"
if PATH="$tmp/bin:$PATH" FAKE_UNAME_M=x86_64 \
  FAKE_TERRAFORM_ARCHIVE="$artifact" RUNNER_TEMP="$unsupported" \
  GITHUB_PATH="$unsupported/github-path" \
  bash scripts/terraform-install.sh "$version" "$real_sum" \
  >"$unsupported/output" 2>&1; then
  echo "FAIL: an unexpected runner architecture must exit non-zero" >&2
  fails=$((fails + 1))
else
  assert "unexpected runner architecture is reported" \
    grep -Fq 'unsupported Terraform CI architecture' "$unsupported/output"
fi

if [ "$fails" -ne 0 ]; then
  echo "$fails check(s) failed" >&2
  exit 1
fi
echo "all terraform-install checks passed"
