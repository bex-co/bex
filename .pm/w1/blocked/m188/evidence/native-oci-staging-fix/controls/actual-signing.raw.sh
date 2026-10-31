syft_work_dir="$(mktemp -d "$RUNNER_TEMP/native-syft.XXXXXX")"
trap 'rm -rf -- "$syft_work_dir"' EXIT
# Staged compressed blobs and Syft's cache share this checked filesystem.
# This is a capacity check, not a reservation against parallel jobs.
available_kib="$(df -Pk "$syft_work_dir" | awk 'NR == 2 {print $4}')"
if ! [[ "$available_kib" =~ ^[0-9]+$ ]] || (( available_kib < 28 * 1024 * 1024 )); then
  echo 'Native SBOM staging needs at least 28 GiB of free space' >&2
  exit 1
fi
# Rekor v2 logs signed PAE hashes; complete predicates remain
# in signed OCI bundles. Exhausted retries stop tag publication.
retry_signing() {
  for attempt in 1 2 3; do
    if timeout --kill-after=10s 5m "$@"; then
      return 0
    fi
    [ "$attempt" -lt 3 ] || return 1
    sleep "$((attempt * 5))"
  done
}
generate_sbom() {
  local arch="$1"
  local output="native-$arch.spdx.json"
  local layout="$syft_work_dir/$arch/layout"
  local cache="$syft_work_dir/$arch/cache"
  local proof="$PWD/native-$arch-oci-stage.json"
  local stage_status=0
  mkdir -p -- "$cache"
  rm -f -- "$output"
  # Retry individual authenticated blobs, retaining verified progress.
  # Syft then reads exact OCI bytes locally, with no registry stream.
  timeout --kill-after=10s 30m python3 .github/scripts/native-oci-stage.py \
    --ref "$REF" --platform "linux/$arch" --output "$layout" --proof "$proof" \
    || stage_status=$?
  if [ -f "$proof" ]; then cat "$proof"; fi
  if [ "$stage_status" -ne 0 ]; then
    echo "Native OCI staging $arch failed with exit $stage_status" >&2
    return "$stage_status"
  fi
  for attempt in 1 2 3; do
    rm -f -- "$output"
    rm -rf -- "${cache:?}"
    mkdir -p -- "$cache"
    if TMPDIR="$cache" GODEBUG=http2client=0 SYFT_CHECK_FOR_APP_UPDATE=false \
      timeout --kill-after=10s 30m syft scan "oci-dir:$layout" \
        --source-name "$NATIVE_IMAGE" --source-version "${REF##*@}" \
        --platform "linux/$arch" -o spdx-json > "$output"; then
      rm -rf -- "${syft_work_dir:?}/${arch:?}"
      return 0
    fi
    echo "Syft $arch attempt $attempt failed" >&2
    rm -f -- "$output"
    [ "$attempt" -lt 3 ] || return 1
    sleep "$((attempt * 5))"
  done
}
retry_signing cosign signing-config create --with-default-rekor-v2 --out "$RUNNER_TEMP/native-signing-config.json"
retry_signing cosign sign --yes --oidc-provider=github-actions --signing-config "$RUNNER_TEMP/native-signing-config.json" "$REF"
for arch in amd64 arm64; do
  generate_sbom "$arch"
  retry_signing cosign attest --yes --oidc-provider=github-actions --signing-config "$RUNNER_TEMP/native-signing-config.json" --type spdxjson --predicate "native-$arch.spdx.json" "$REF"
  retry_signing cosign attest --yes --oidc-provider=github-actions --signing-config "$RUNNER_TEMP/native-signing-config.json" --type https://aquasecurity.github.io/trivy/attestation/vuln/v1 --predicate "native-$arch.trivy.json" "$REF"
done
# Only verified, scanned and signed candidates reach this discovery
# tag. Adopting its immutable digest still requires a reviewed commit.
docker buildx imagetools create --tag "${{ env.NATIVE_IMAGE }}:native-${{ matrix.name }}-${GITHUB_SHA}" "$REF"
docker buildx imagetools create --tag "${{ env.NATIVE_IMAGE }}:native-${{ matrix.name }}" "$REF"
echo "${{ matrix.base }} -> $REF (producer $PRODUCER; verification $GITHUB_SHA)" >> "$GITHUB_STEP_SUMMARY"
