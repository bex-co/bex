#!/usr/bin/env bash
# Offline tests for the rootless HashiCorp (Terraform, Packer) installer. The release download and
# platform probe are faked; ZIP creation/extraction and SHA-256 verification are
# real, including the failure paths that must install nothing.
set -euo pipefail
cd "$(dirname "$0")/.."

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
mkdir -p "$tmp/bin" "$tmp/release"

version=9.9.9

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
cp "$FAKE_HASHICORP_ARCHIVE" "$destination"
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

sha256() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1" | awk '{print $1}'
  else
    shasum -a 256 "$1" | awk '{print $1}'
  fi
}

# run_installer <product> <archive> <runner-temp> <checksum>
run_installer() {
  local product="$1" archive="$2" runner_temp="$3" checksum="$4"
  mkdir -p "$runner_temp"
  : >"$runner_temp/github-path"
  PATH="$tmp/bin:$PATH" \
    FAKE_HASHICORP_ARCHIVE="$archive" \
    RUNNER_TEMP="$runner_temp" \
    GITHUB_PATH="$runner_temp/github-path" \
    bash scripts/hashicorp-install.sh "$product" "$version" "$checksum" \
    >"$runner_temp/output" 2>&1
}

# check_product <product> <banner its `version` prints>
check_product() {
  local product="$1" banner="$2"
  local release="$tmp/release/$product"
  local artifact="$release/${product}_${version}_linux_arm64.zip"
  mkdir -p "$release"
  printf '#!/usr/bin/env bash\necho "%s"\n' "$banner" >"$release/$product"
  chmod +x "$release/$product"
  python3 - "$release/$product" "$product" "$artifact" <<'PY'
import sys
from zipfile import ZIP_DEFLATED, ZipFile

source, name, artifact = sys.argv[1:]
with ZipFile(artifact, "w", compression=ZIP_DEFLATED) as archive:
    archive.write(source, name)
PY
  local real_sum
  real_sum="$(sha256 "$artifact")"

  echo "hashicorp-install.sh $product"

  local happy="$tmp/$product-happy"
  if run_installer "$product" "$artifact" "$happy" "$real_sum"; then
    assert "verified $product is installed" test -x "$happy/${product}-${version}/$product"
    assert "the install directory is exported to GITHUB_PATH" \
      grep -Fxq "$happy/${product}-${version}" "$happy/github-path"
    assert "the installed binary was executed" grep -Fq "$banner" "$happy/output"
  else
    echo "FAIL: verified $product install exited non-zero" >&2
    fails=$((fails + 1))
  fi

  local bad_sum=0000000000000000000000000000000000000000000000000000000000000000
  local mismatch="$tmp/$product-mismatch"
  if run_installer "$product" "$artifact" "$mismatch" "$bad_sum"; then
    echo "FAIL: checksum mismatch must exit non-zero" >&2
    fails=$((fails + 1))
  else
    assert "a mismatched release is not installed" test ! -e "$mismatch/${product}-${version}/$product"
    assert "the checksum mismatch is reported" grep -Fq 'checksum mismatch' "$mismatch/output"
  fi

  local unsupported="$tmp/$product-unsupported"
  if FAKE_UNAME_M=x86_64 run_installer "$product" "$artifact" "$unsupported" "$real_sum"; then
    echo "FAIL: an unexpected runner architecture must exit non-zero" >&2
    fails=$((fails + 1))
  else
    assert "unexpected runner architecture is reported" \
      grep -Fq "unsupported $product CI architecture" "$unsupported/output"
  fi
}

check_product terraform "Terraform v9.9.9"
check_product packer "Packer v9.9.9"

echo "hashicorp-install.sh (unknown product)"
unknown="$tmp/unknown"
if run_installer vault /dev/null "$unknown" 0000000000000000000000000000000000000000000000000000000000000000; then
  echo "FAIL: a product outside the allowlist must exit non-zero" >&2
  fails=$((fails + 1))
else
  assert "a product outside the allowlist is refused" grep -Fq 'usage:' "$unknown/output"
fi

if [ "$fails" -ne 0 ]; then
  echo "$fails check(s) failed" >&2
  exit 1
fi
echo "all hashicorp-install checks passed"
