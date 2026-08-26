#!/usr/bin/env bash
# Install the reviewed Terraform release on the deliberately minimal,
# unprivileged Linux/ARM64 Actions runners. hashicorp/setup-terraform delegates
# ZIP extraction to a host `unzip` executable, which these runners do not carry;
# use Python's standard library so installation needs neither sudo nor mutable
# runner state.
#
# Usage: bash scripts/terraform-install.sh <version> <linux-arm64-sha256>
set -euo pipefail

version="${1:-}"
want="${2:-}"

if ! [[ "$version" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  echo "usage: $0 <version> <linux-arm64-sha256>" >&2
  exit 1
fi
if ! [[ "$want" =~ ^[0-9a-f]{64}$ ]]; then
  echo "a lowercase 64-hex Terraform SHA-256 is required" >&2
  exit 1
fi
if [ "$(uname -s)" != Linux ]; then
  echo "unsupported Terraform CI operating system: $(uname -s)" >&2
  exit 1
fi
case "$(uname -m)" in
arm64 | aarch64) ;;
*)
  echo "unsupported Terraform CI architecture: $(uname -m)" >&2
  exit 1
  ;;
esac

: "${RUNNER_TEMP:?RUNNER_TEMP is required}"
: "${GITHUB_PATH:?GITHUB_PATH is required}"

artifact="terraform_${version}_linux_arm64.zip"
download_dir="$(mktemp -d "$RUNNER_TEMP/terraform-download.XXXXXX")"
install_dir="$RUNNER_TEMP/terraform-${version}"
cleanup() { rm -rf -- "$download_dir"; }
trap cleanup EXIT

echo "downloading ${artifact}"
curl -fsSLo "$download_dir/$artifact" \
  "https://releases.hashicorp.com/terraform/${version}/${artifact}"

if command -v sha256sum >/dev/null 2>&1; then
  got="$(sha256sum "$download_dir/$artifact" | awk '{print $1}')"
else
  got="$(shasum -a 256 "$download_dir/$artifact" | awk '{print $1}')"
fi
if [ "$got" != "$want" ]; then
  echo "Terraform checksum mismatch for ${artifact}" >&2
  echo "  expected: $want" >&2
  echo "  received: $got" >&2
  exit 1
fi

install -d "$install_dir"
if ! python3 - "$download_dir/$artifact" "$install_dir/terraform" <<'PY'
import shutil
import sys
from zipfile import BadZipFile, ZipFile

archive_path, target = sys.argv[1:]
try:
    with ZipFile(archive_path) as archive:
        member = archive.getinfo("terraform")
        if member.is_dir():
            raise KeyError("terraform is a directory")
        with archive.open(member) as source, open(target, "wb") as output:
            shutil.copyfileobj(source, output)
except (BadZipFile, KeyError, OSError):
    raise SystemExit(1)
PY
then
  echo "Terraform archive did not contain the expected binary" >&2
  exit 1
fi

chmod 755 "$install_dir/terraform"
"$install_dir/terraform" version
printf '%s\n' "$install_dir" >>"$GITHUB_PATH"
