#!/usr/bin/env bash
# rust-line-smoke.sh <rust:1-bookworm@sha256:…> <rust release>
#
# Before rust-line-bump.yml pins a new native Rust image (w3/m638), prove on
# this runner's architecture that the image is the release the plan read from
# its config, that rustup's default toolchain is that release with nothing
# overriding it, and that cargo builds a crate declaring it as its
# rust-version with the network off. Cargo refuses a rust-version newer than
# the active rustc, so a crate the image's release cannot build fails here, and
# nothing may be downloaded instead. The source is mounted read-only and copied
# inside the container, because cargo writes Cargo.lock and target/ beside the
# manifest. The workflow runs it on ubuntu-24.04-arm and on ubuntu-latest, the
# two architectures the official index publishes and native builds run on.
set -euo pipefail

usage="usage: rust-line-smoke.sh <rust:1-bookworm@sha256:…> <rust release>"
image="${1:?$usage}"
release="${2:?$usage}"
if ! [[ "$image" =~ ^rust:1-bookworm@sha256:[0-9a-f]{64}$ ]]; then
  echo "FAIL: $image is not a digest-pinned rust:1-bookworm image" >&2
  exit 1
fi
if ! [[ "$release" =~ ^1\.[0-9]+\.[0-9]+$ ]]; then
  echo "FAIL: $release is not a Rust 1.x release (want 1.minor.patch)" >&2
  exit 1
fi
case "$(uname -m)" in
  x86_64 | amd64) arch=amd64 triple=x86_64-unknown-linux-gnu ;;
  aarch64 | arm64) arch=arm64 triple=aarch64-unknown-linux-gnu ;;
  *)
    echo "FAIL: no official rust image for host $(uname -m)" >&2
    exit 1
    ;;
esac

# Docker Hub pulls stall now and then; the run itself must not be the retry.
for attempt in 1 2 3; do
  docker pull --quiet --platform "linux/$arch" "$image" >/dev/null && break
  if [ "$attempt" -eq 3 ]; then
    echo "FAIL: could not pull $image for linux/$arch" >&2
    exit 1
  fi
  sleep $((attempt * ${RUST_SMOKE_RETRY_PAUSE:-10}))
done

work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT
mkdir -p "$work/src"
printf '[package]\nname = "smoke"\nversion = "0.1.0"\nedition = "2021"\nrust-version = "%s"\n' "$release" >"$work/Cargo.toml"
printf 'fn main() {\n    println!("smoke ok");\n}\n' >"$work/src/main.rs"
set +e
out="$(docker run --rm --platform "linux/$arch" --network none -v "$work:/src:ro" "$image" \
  sh -c 'rustc --version && rustup show active-toolchain && cp -R /src /tmp/smoke && cd /tmp/smoke &&
    cargo build --offline --quiet && ./target/debug/smoke' 2>&1)"
rc=$?
set -e
# rustc names its commit and date after the release, which the plan does not know.
re_release="${release//./\\.}"
want="^rustc $re_release \([0-9a-f]+ [0-9]{4}-[0-9]{2}-[0-9]{2}\)
$re_release-$triple \(default\)
smoke ok$"
if [ "$rc" -ne 0 ] || ! [[ "$out" =~ $want ]]; then
  echo "FAIL: $image on linux/$arch (exit $rc) printed:" >&2
  printf '%s\n' "$out" >&2
  echo "want (regex):" >&2
  printf '%s\n' "$want" >&2
  exit 1
fi
echo "ok: $image is Rust $release on linux/$arch and builds a rust-version $release crate offline"
