#!/bin/sh
# Install the bex CLI from GitHub releases.
#   curl -fsSL https://raw.githubusercontent.com/bex-co/bex/main/scripts/install-bex.sh | sh
#
# Environment overrides:
#   BEX_VERSION       exact version to install (X.Y.Z); default: newest bex-cli/v* release
#   BEX_INSTALL_DIR   target directory; default ~/.local/bin
#   GITHUB_TOKEN      used for the release-list API call when set (avoids the
#                     unauthenticated rate limit on shared egress IPs / CI)
#   BEX_API_URL       GitHub API origin (tests); default https://api.github.com
#   BEX_DOWNLOAD_URL  release-asset origin (tests); default https://github.com/bex-co/bex/releases/download
set -eu

repo="bex-co/bex"
api_url="${BEX_API_URL:-https://api.github.com}"
download_url="${BEX_DOWNLOAD_URL:-https://github.com/${repo}/releases/download}"
install_dir="${BEX_INSTALL_DIR:-$HOME/.local/bin}"

fail() { echo "install-bex: error: $*" >&2; exit 1; }

command -v curl >/dev/null || fail "curl is required"
command -v tar >/dev/null || fail "tar is required"
command -v awk >/dev/null || fail "awk is required"

case "$(uname -s)" in
  Darwin) os="darwin" ;;
  Linux) os="linux" ;;
  *) fail "unsupported OS $(uname -s) (bex ships darwin and linux builds)" ;;
esac
case "$(uname -m)" in
  arm64 | aarch64) arch="arm64" ;;
  x86_64 | amd64) arch="amd64" ;;
  *) fail "unsupported architecture $(uname -m) (bex ships arm64 and amd64 builds)" ;;
esac

if command -v sha256sum >/dev/null; then
  sha256_tool="sha256sum"
elif command -v shasum >/dev/null; then
  sha256_tool="shasum -a 256"
else
  fail "need sha256sum or shasum to verify the download"
fi

workdir="$(mktemp -d)"
cleanup() { rm -rf "$workdir"; }
trap cleanup EXIT
trap 'cleanup; exit 130' INT
trap 'cleanup; exit 143' TERM HUP

version="${BEX_VERSION:-}"
if [ -z "$version" ]; then
  # The bex monorepo releases several components; releases/latest is not
  # necessarily a CLI release, so take the first (= newest-published)
  # bex-cli/v* tag in the list. /release only publishes stable CLI tags, so
  # publish order is version order; the 100-release horizon is accepted.
  if [ -n "${GITHUB_TOKEN:-}" ]; then
    curl -fsSL -H 'Accept: application/vnd.github+json' \
      -H "Authorization: Bearer ${GITHUB_TOKEN}" \
      "${api_url}/repos/${repo}/releases?per_page=100" > "${workdir}/releases.json" ||
      fail "listing releases failed (${api_url})"
  else
    curl -fsSL -H 'Accept: application/vnd.github+json' \
      "${api_url}/repos/${repo}/releases?per_page=100" > "${workdir}/releases.json" ||
      fail "listing releases failed (${api_url}; if rate-limited, set GITHUB_TOKEN or BEX_VERSION)"
  fi
  version="$(grep -o '"tag_name": *"bex-cli/v[^"]*"' "${workdir}/releases.json" |
    head -n 1 | sed 's/.*bex-cli\/v//; s/"$//')"
  [ -n "$version" ] || fail "no bex-cli/v* release found via ${api_url}"
fi

artifact="bex-${version}-${os}-${arch}"
# The tag contains a slash; GitHub encodes it as %2F in asset URLs.
asset_base="${download_url}/bex-cli%2Fv${version}"

echo "Downloading bex v${version} (${os}/${arch})…"
# checksums.txt first: fail before the larger download when the release is
# malformed or has no entry for this platform. Authenticity requires the
# Sigstore bundle (same policy as `bex upgrade`); a matching unsigned
# checksum alone is not enough (codex round-16 #1).
curl -fsSL -o "${workdir}/checksums.txt" "${asset_base}/checksums.txt" ||
  fail "download failed: ${asset_base}/checksums.txt"
curl -fsSL -o "${workdir}/checksums.txt.sigstore.json" "${asset_base}/checksums.txt.sigstore.json" ||
  fail "download failed: ${asset_base}/checksums.txt.sigstore.json (unsigned releases are refused)"

if ! command -v cosign >/dev/null; then
  fail "cosign is required to verify the release signature (install from https://docs.sigstore.dev/cosign/system_config/installation/)"
fi
cosign verify-blob \
  --bundle "${workdir}/checksums.txt.sigstore.json" \
  --certificate-identity-regexp '^https://github\.com/bex-co/bex/\.github/workflows/cli-release\.yml@refs/tags/bex-cli/v' \
  --certificate-oidc-issuer 'https://token.actions.githubusercontent.com' \
  "${workdir}/checksums.txt" >/dev/null ||
  fail "checksums.txt signature verification failed — refusing to install"

expected="$(awk -v f="${artifact}.tar.gz" '$2 == f { print $1; exit }' "${workdir}/checksums.txt")"
[ -n "$expected" ] || fail "checksums.txt has no entry for ${artifact}.tar.gz"

curl -fsSL -o "${workdir}/${artifact}.tar.gz" "${asset_base}/${artifact}.tar.gz" ||
  fail "download failed: ${asset_base}/${artifact}.tar.gz"
actual="$(cd "$workdir" && $sha256_tool "${artifact}.tar.gz" | awk '{print $1}')"
[ "$expected" = "$actual" ] || fail "checksum mismatch for ${artifact}.tar.gz (expected ${expected}, got ${actual})"

tar -xzf "${workdir}/${artifact}.tar.gz" -C "$workdir"
[ -f "${workdir}/${artifact}/bex" ] || fail "archive did not contain ${artifact}/bex"

mkdir -p "$install_dir"
# Stage in the destination directory, then rename: an interrupted copy must
# never leave a truncated-but-executable bex on PATH.
install -m 0755 "${workdir}/${artifact}/bex" "${install_dir}/.bex.tmp.$$"
mv -f "${install_dir}/.bex.tmp.$$" "${install_dir}/bex"

echo "Installed bex v${version} to ${install_dir}/bex"
case ":$PATH:" in
  *":${install_dir}:"*) ;;
  *) echo "Note: ${install_dir} may not be on your PATH — if \`bex\` is not found, add it:
  export PATH=\"${install_dir}:\$PATH\"" ;;
esac
"${install_dir}/bex" -v || true
