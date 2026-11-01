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
  for attempt in 1 2 3; do
    rm -f -- "$output"
    if GODEBUG=http2client=0 timeout --kill-after=10s 30m syft "registry:$REF" --platform "linux/$arch" -o spdx-json > "$output"; then
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
