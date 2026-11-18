#!/usr/bin/env bash
set -euo pipefail

# Render Terraform provider compatibility (w9/m474): run render-oss/
# terraform-provider-render's own acceptance tests, unmodified and pinned, in
# go-vcr record mode (UPDATE_RECORDINGS=true) against a bex API, one resource
# package at a time, and print a per-test result table. Grades go to
# docs/terraform-compatibility-checklist.md.
#
# Usage: scripts/terraform-provider-verify.sh <package>... | all
#
# Required:
#   BEX_API_URL      bex-api origin, e.g. https://api.bex.co or
#                    http://localhost:54090 (a trailing /v1 is accepted)
#   RENDER_API_KEY   a bex API key (bearer form) for the workspace
#   RENDER_OWNER_ID  that workspace's tea- id
# Optional:
#   TF_VERIFY_RUN      run only the tests whose name matches this grep -E
#                      expression (default: every test)
#   TF_VERIFY_WORKDIR  clone, Terraform, logs and recordings (default: a new
#                      mktemp -d); must be outside this repository
#   TF_VERIFY_TIMEOUT  go test -timeout per test (default 60m)
#
# The tests create real services, datastores, env groups, projects and
# webhooks. Terraform destroys them at the end of each test; after it, every
# resource that appeared in the workspace while the test ran is deleted too, so
# use a workspace nobody else is creating resources in. Re-recorded
# cassettes and logs are run output: they stay in the work directory, never in
# git, and the script prints their sha256.

provider_repo=https://github.com/render-oss/terraform-provider-render.git
provider_commit=06af456d9bdb1b839f29923ac7e435c4a1f01f3c # tip on 2026-11-16
terraform_version=1.17.2
terraform_sha256() {
  case "$1" in
  darwin_arm64) echo da0c7ef278a911347862758e7573a5907365e66485c2637cf5c217c5e227992e ;;
  darwin_amd64) echo 66b472dcc36cf770d83757ed4d002c476174842c05c68637b40a7e99519cf3e1 ;;
  linux_amd64) echo e6f16b2f0b137ee5729caa8c4e79a3bae4ee261acd198a1fcfa8584a6296605b ;;
  linux_arm64) echo 858e08c181a76b6b223e8c767a8e983922eebe2d062db949a2de66ce040c368c ;;
  *) return 1 ;;
  esac
}
# The provider's resource packages (internal/provider/<name>), each one row of
# the checklist.
packages=(backgroundworker cronjob dedicatedip envgroup keyvalue logstreams
  metricstream notifications postgres privateservice project redis
  registrycredential staticsite webhook webservice)

usage() {
  echo "usage: scripts/terraform-provider-verify.sh <package>... | all" >&2
  echo "packages: ${packages[*]}" >&2
  exit 2
}

repo_root="$(cd "$(dirname "$0")/.." && pwd -P)"
helper="$repo_root/scripts/terraform-provider-verify/helper.py"

[ "$#" -gt 0 ] || usage
selected=()
for arg in "$@"; do
  if [ "$arg" = all ]; then
    selected=("${packages[@]}")
    break
  fi
  known=0
  for p in "${packages[@]}"; do [ "$p" = "$arg" ] && known=1; done
  [ "$known" = 1 ] || { echo "error: unknown provider package '$arg'" >&2; usage; }
  selected+=("$arg")
done

: "${BEX_API_URL:?set BEX_API_URL to the bex-api origin}"
: "${RENDER_API_KEY:?set RENDER_API_KEY to a bex API key}"
: "${RENDER_OWNER_ID:?set RENDER_OWNER_ID to the workspace tea- id}"
api_origin="${BEX_API_URL%/}"
api_origin="${api_origin%/v1}"
export BEX_API_URL="$api_origin"

# physical_path P: P with symlinks resolved, for a P that may not exist yet
# (its nearest existing ancestor is resolved), so the repository check runs
# before anything is created.
physical_path() {
  local dir="$1" rest=""
  case "$dir" in /*) ;; *) dir="$PWD/$dir" ;; esac
  while [ ! -d "$dir" ]; do
    rest="/$(basename "$dir")$rest"
    dir="$(dirname "$dir")"
  done
  echo "$(cd "$dir" && pwd -P)$rest"
}
work="$(physical_path "${TF_VERIFY_WORKDIR:-$(mktemp -d "${TMPDIR:-/tmp}/terraform-provider-verify.XXXXXX")}")"
case "$work/" in
"$repo_root"/*)
  echo "error: TF_VERIFY_WORKDIR $work is inside the repository; recordings and logs must stay out of git" >&2
  exit 2
  ;;
esac
mkdir -p "$work/logs" "$work/state"
echo "==> work directory: $work"

# Terraform: the pinned release, checked against the pinned checksum.
os="$(uname -s | tr '[:upper:]' '[:lower:]')"
arch="$(uname -m)"
case "$arch" in x86_64) arch=amd64 ;; aarch64) arch=arm64 ;; esac
terraform="$work/terraform/terraform"
if [ ! -x "$terraform" ] || ! "$terraform" version | grep -qx "Terraform v$terraform_version"; then
  want="$(terraform_sha256 "${os}_$arch")" || { echo "error: no pinned Terraform checksum for ${os}_$arch" >&2; exit 1; }
  rm -rf "$work/terraform"
  mkdir -p "$work/terraform"
  zip="$work/terraform/terraform.zip"
  curl -sSfL -o "$zip" "https://releases.hashicorp.com/terraform/$terraform_version/terraform_${terraform_version}_${os}_$arch.zip"
  got="$(shasum -a 256 "$zip" | cut -d' ' -f1)"
  [ "$got" = "$want" ] || { echo "error: Terraform zip sha256 $got, want $want" >&2; exit 1; }
  unzip -oq "$zip" terraform -d "$work/terraform"
fi
echo "==> $("$terraform" version | head -1)"

# The provider: a fresh clone at the pinned commit. It is downloaded code, so
# nothing from it runs except `go test` on the packages named here (no make,
# no scripts). The directory keeps the repository's name: the tests find
# their example configurations by walking up to a "terraform-provider-render"
# directory (internal/provider/testhelpers/path.go).
src="$work/terraform-provider-render"
if [ ! -d "$src/.git" ]; then
  git init -q "$src"
  git -C "$src" fetch -q --depth 1 "$provider_repo" "$provider_commit"
  git -C "$src" -c advice.detachedHead=false checkout -q FETCH_HEAD
fi
checked_out="$(git -C "$src" rev-parse HEAD)"
[ "$checked_out" = "$provider_commit" ] || { echo "error: $src is at $checked_out, want $provider_commit" >&2; exit 1; }
echo "==> render-oss/terraform-provider-render $provider_commit"

echo
echo "| package | test | target | result | seconds | first error |"
echo "| --- | --- | --- | --- | --- | --- |"
status=0
for pkg in "${selected[@]}"; do
  # Start from the provider's own cassettes, so the diff after the run is
  # exactly Render's recording against bex's.
  git -C "$src" checkout -q HEAD -- "internal/provider/$pkg"
  for f in json stderr sweep; do : >"$work/logs/$pkg.$f"; done
  # One test per `go test`, each followed by a sweep: a resource bex created
  # but the provider lost from its state (it failed reading it back) is
  # deleted before the next test reuses its name.
  while IFS=$'\t' read -r dir test target; do
    if [ -n "${TF_VERIFY_RUN:-}" ] && ! grep -Eq -- "$TF_VERIFY_RUN" <<<"$test"; then continue; fi
    [ "$target" = offline ] || python3 -I "$helper" snapshot "$work/state/before.json" 2>>"$work/logs/$pkg.sweep"
    GOWORK=off GOFLAGS=-mod=readonly CHECKPOINT_DISABLE=1 TF_IN_AUTOMATION=1 \
      TF_ACC=1 UPDATE_RECORDINGS=true TF_ACC_TERRAFORM_PATH="$terraform" \
      RENDER_HOST="$api_origin/v1" RENDER_API_KEY="$RENDER_API_KEY" RENDER_OWNER_ID="$RENDER_OWNER_ID" \
      go -C "$src" test -count=1 -json -timeout "${TF_VERIFY_TIMEOUT:-60m}" -run "^$test\$" \
      "./internal/provider/$pkg/$dir" >>"$work/logs/$pkg.json" 2>>"$work/logs/$pkg.stderr" || true
    [ "$target" != offline ] || continue
    echo "== $pkg/$dir $test" >>"$work/logs/$pkg.sweep"
    python3 -I "$helper" sweep "$work/state/before.json" >>"$work/logs/$pkg.sweep" 2>&1 || {
      status=1
      echo "error: $pkg/$dir $test left resources the sweep could not delete (see $work/logs/$pkg.sweep)" >&2
    }
  done < <(python3 -I "$helper" tests "$src" "$pkg")
  python3 -I "$helper" summarize "$src" "$pkg" "$work/logs/$pkg.json" || status=1
done

echo
echo "==> resources a test left behind, deleted by the sweep:"
for pkg in "${selected[@]}"; do
  awk '/^== / { test = substr($0, 4) } /^  swept / { print "  " test ":" substr($0, 8) }' "$work/logs/$pkg.sweep"
done

tarball="$work/recordings.tar.gz"
{
  git -C "$src" diff --name-only -z -- internal/provider
  git -C "$src" ls-files -z --others --exclude-standard -- internal/provider
} >"$work/state/recorded.list"
tar -czf "$tarball" -C "$src" --null -T "$work/state/recorded.list"
echo
echo "==> logs: $work/logs (sweeps in *.sweep)"
echo "==> recordings: $tarball sha256 $(shasum -a 256 "$tarball" | cut -d' ' -f1)"
echo "    compare with Render's: git -C $src diff -- internal/provider/<package>"
exit "$status"
