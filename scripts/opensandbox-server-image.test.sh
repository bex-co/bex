#!/usr/bin/env bash
# Tests for scripts/opensandbox-server-image.sh (w8/254): a deploy reuses the
# OpenSandbox lifecycle server image built from the same inputs instead of
# pinning a new digest, and so restarting the one-replica server, every time.
# No registry and no scanner: docker and trivy are stubs on PATH.
#
#   bash scripts/opensandbox-server-image.test.sh
set -uo pipefail
cd "$(dirname "$0")/.." || exit 1

SCRIPT="scripts/opensandbox-server-image.sh"
WORKFLOW=".github/workflows/deploy.yml"
IMAGE="ghcr.io/example/opensandbox-server"
DIGEST="sha256:$(printf 'a%.0s' {1..64})"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT
STUBS="$TMP/bin"
LOG="$TMP/calls.log"
CONTEXT="$TMP/context"
mkdir -p "$STUBS" "$CONTEXT"
cp deploy/opensandbox/server.Dockerfile deploy/opensandbox/requirements.lock "$CONTEXT/"

# docker: answers `buildx imagetools inspect` with the shape buildx prints for
# an image index (the top-level digest, then the per-platform manifests).
cat >"$STUBS/docker" <<'EOF_STUB'
#!/usr/bin/env bash
printf 'docker %s\n' "$*" >>"$STUB_LOG"
[ "$1 $2 $3" = "buildx imagetools inspect" ] || { echo "stub docker: unexpected call: $*" >&2; exit 99; }
case "${STUB_PUBLISHED:-none}" in
  none) echo "ERROR: $4: not found" >&2; exit 1 ;;
  garbage) echo "<html>bad gateway</html>"; exit 0 ;;
  nodigest) echo '{"schemaVersion":2,"manifests":[]}'; exit 0 ;;
  *)
    printf '{\n  "schemaVersion": 2,\n  "mediaType": "application/vnd.oci.image.index.v1+json",\n  "digest": "%s",\n  "size": 856,\n  "manifests": [\n    {"mediaType": "application/vnd.oci.image.manifest.v1+json", "digest": "sha256:%s", "size": 1}\n  ]\n}\n' \
      "$STUB_PUBLISHED" "$(printf 'b%.0s' {1..64})"
    exit 0
    ;;
esac
EOF_STUB
# trivy: prints a report on stdout, as the real one does, and fails the gate
# when told to.
cat >"$STUBS/trivy" <<'EOF_STUB'
#!/usr/bin/env bash
printf 'trivy %s\n' "$*" >>"$STUB_LOG"
echo "Report Summary: stub table"
exit "${STUB_TRIVY_RC:-0}"
EOF_STUB
# cosign: verifies when told the image carries the expected signer.
cat >"$STUBS/cosign" <<'EOF_STUB'
#!/usr/bin/env bash
printf 'cosign %s\n' "$*" >>"$STUB_LOG"
exit "${STUB_COSIGN_RC:-0}"
EOF_STUB
chmod +x "$STUBS/docker" "$STUBS/trivy" "$STUBS/cosign"
SIGNER="https://github.com/example/repo/.github/workflows/deploy.yml@refs/heads/main"

FAILURES=0
ok() { echo "ok   $1"; }
bad() {
  echo "FAIL $1"
  FAILURES=$((FAILURES + 1))
}

# decide [ENV=VALUE ...]: run the script against the stubs; sets $out (stdout
# only, what a deploy appends to $GITHUB_OUTPUT), $err and $rc.
decide() {
  : >"$LOG"
  out="$(env PATH="$STUBS:$PATH" STUB_LOG="$LOG" BEX_OPENSANDBOX_CONTEXT="$CONTEXT" \
    BEX_IMAGE_REFRESH_STAMP=2026-W46 "$@" bash "$SCRIPT" "$IMAGE" "${VERSION:-0.2.2}" "${PLATFORM:-linux/amd64}" "$SIGNER" 2>"$TMP/stderr")"
  rc=$?
  err="$(cat "$TMP/stderr")"
}
tag_of() { sed -n 's/^tag=//p' <<<"$out"; }

# --- nothing published for these inputs: build ---
decide
tag="$(tag_of)"
[ "$rc" -eq 0 ] && [[ "$tag" =~ ^inputs-[0-9a-f]{16}$ ]] \
  && ok "names the inputs with a tag ($tag)" \
  || bad "no inputs tag (exit $rc): $out"
[ "$out" = "tag=$tag"$'\n'"digest=" ] \
  && ok "nothing published for the inputs means build" \
  || bad "expected an empty digest, got: $out"
grep -qF "docker buildx imagetools inspect $IMAGE:$tag " "$LOG" \
  && ok "looks the inputs tag up in the registry" \
  || bad "did not inspect $IMAGE:$tag"
grep -q '^trivy ' "$LOG" \
  && bad "scanned an image that does not exist" \
  || ok "does not scan when there is nothing to reuse"

# --- published and still passing the gate: reuse ---
decide STUB_PUBLISHED="$DIGEST"
[ "$rc" -eq 0 ] && [ "$out" = "tag=$tag"$'\n'"digest=$DIGEST" ] \
  && ok "reuses the published image by its index digest" \
  || bad "expected digest=$DIGEST (exit $rc), got: $out"
grep -qF "cosign verify --certificate-identity $SIGNER --certificate-oidc-issuer https://token.actions.githubusercontent.com $IMAGE@$DIGEST" "$LOG" \
  && ok "verifies the candidate was signed by this workflow on main" \
  || bad "the candidate's signer was not verified"
grep -qF "trivy image --exit-code 3 --severity CRITICAL --ignore-unfixed $IMAGE@$DIGEST" "$LOG" \
  && ok "scans the candidate with the deploy's CRITICAL gate" \
  || bad "the candidate was not scanned as the CRITICAL gate scans"
grep -qF "Report Summary" <<<"$out" \
  && bad "the scan report leaked into the step outputs" \
  || ok "the scan report stays out of the step outputs"

# --- published but not ours, failing the gate, or unscannable: build ---
decide STUB_PUBLISHED="$DIGEST" STUB_COSIGN_RC=1
[ "$rc" -eq 0 ] && [ "$out" = "tag=$tag"$'\n'"digest=" ] && grep -qF "is not signed by" <<<"$err" && ! grep -q '^trivy ' "$LOG" \
  && ok "an image this workflow did not sign on main is never reused" \
  || bad "a foreign image was reused or scanned (exit $rc): $out"
decide STUB_PUBLISHED="$DIGEST" STUB_TRIVY_RC=3
[ "$rc" -eq 0 ] && [ "$out" = "tag=$tag"$'\n'"digest=" ] && grep -qF "no longer passes the CRITICAL CVE gate" <<<"$err" \
  && ok "an image that fails the CRITICAL gate is rebuilt" \
  || bad "a failing image was kept (exit $rc): $out"
decide STUB_PUBLISHED="$DIGEST" STUB_TRIVY_RC=1
[ "$rc" -eq 0 ] && [ "$out" = "tag=$tag"$'\n'"digest=" ] && grep -qF "scanning $IMAGE@$DIGEST failed (trivy exit 1)" <<<"$err" \
  && ok "a scan that fails to run means build, and says so" \
  || bad "a failed scan was not reported as one (exit $rc): $out ($err)"
for broken in garbage nodigest "sha256:abc" "md5:$(printf 'a%.0s' {1..64})"; do
  decide STUB_PUBLISHED="$broken"
  [ "$rc" -eq 0 ] && [ "$out" = "tag=$tag"$'\n'"digest=" ] && ! grep -q '^trivy ' "$LOG" \
    && ok "an unusable registry answer ($broken) means build" \
    || bad "registry answer '$broken' was not treated as build (exit $rc): $out"
done

# --- the key follows the inputs ---
decide
[ "$(tag_of)" = "$tag" ] && ok "the same inputs give the same tag" || bad "the tag is not stable"
decide BEX_IMAGE_REFRESH_STAMP=2026-W47
[ "$(tag_of)" != "$tag" ] && ok "a new week gives a new tag" || bad "the week is not in the key"
VERSION=0.2.3 decide
[ "$(tag_of)" != "$tag" ] && ok "a new server version gives a new tag" || bad "the version is not in the key"
PLATFORM=linux/arm64 decide
[ "$(tag_of)" != "$tag" ] && ok "another platform gives a new tag" || bad "the platform is not in the key"
echo "COPY patches/fix.py /tmp/fix.py" >>"$CONTEXT/server.Dockerfile"
decide STUB_PUBLISHED="$DIGEST"
[ "$rc" -eq 0 ] && grep -q '^digest=$' <<<"$out" && grep -qF "copies files the reuse key does not cover" <<<"$err" \
  && ok "a Dockerfile copying an input the key does not cover always builds" \
  || bad "an uncovered COPY was reused (exit $rc): $out"
cp deploy/opensandbox/server.Dockerfile "$CONTEXT/server.Dockerfile"
echo "# a comment" >>"$CONTEXT/server.Dockerfile"
decide
[ "$(tag_of)" != "$tag" ] && ok "a changed Dockerfile gives a new tag" || bad "the Dockerfile is not in the key"
cp deploy/opensandbox/server.Dockerfile "$CONTEXT/server.Dockerfile"
echo "extra==1.0" >>"$CONTEXT/requirements.lock"
decide
[ "$(tag_of)" != "$tag" ] && ok "a changed dependency lock gives a new tag" || bad "the lock is not in the key"
cp deploy/opensandbox/requirements.lock "$CONTEXT/requirements.lock"

# Without the override the refresh stamp is the current ISO week (UTC). Take
# the week on both sides of the run so a test at the turn of a week holds.
before="$(date -u +%G-W%V)"
unset_stamp="$(env -u BEX_IMAGE_REFRESH_STAMP PATH="$STUBS:$PATH" STUB_LOG="$LOG" BEX_OPENSANDBOX_CONTEXT="$CONTEXT" \
  bash "$SCRIPT" "$IMAGE" 0.2.2 linux/amd64 "$SIGNER" 2>/dev/null | sed -n 's/^tag=//p')"
after="$(date -u +%G-W%V)"
match=0
for week in "$before" "$after"; do
  decide BEX_IMAGE_REFRESH_STAMP="$week"
  [ "$(tag_of)" = "$unset_stamp" ] && match=1
done
[ "$match" -eq 1 ] && ok "the refresh stamp defaults to the ISO week" || bad "the default refresh stamp is not the ISO week"

rm "$CONTEXT/requirements.lock"
decide
[ "$rc" -ne 0 ] && ! grep -q '^digest=sha256' <<<"$out" \
  && ok "a missing build input fails instead of keying on nothing" \
  || bad "a missing build input was not refused (exit $rc): $out"
cp deploy/opensandbox/requirements.lock "$CONTEXT/requirements.lock"

# --- the deploy uses it ---
reuse_line="$(grep -n -- '- id: opensandbox_reuse' "$WORKFLOW" | head -1 | cut -d: -f1)"
grep -qF 'run: bash scripts/opensandbox-server-image.sh "$OPENSANDBOX_IMAGE" 0.2.2 linux/amd64 "$SIGNER" >> "$GITHUB_OUTPUT"' "$WORKFLOW" \
  && ok "deploy.yml asks the script before building" \
  || bad "deploy.yml does not run $SCRIPT"
grep -qF 'SIGNER: https://github.com/${{ github.repository }}/.github/workflows/deploy.yml@refs/heads/main' "$WORKFLOW" \
  && ok "the reuse step trusts only this workflow's runs on main" \
  || bad "the reuse step's signer is not this workflow on main"
reuse_block="$(awk '/- id: opensandbox_reuse/{take=1} take{print} take && /run: bash scripts\/opensandbox-server-image.sh/{exit}' "$WORKFLOW")"
grep -qF 'TRIVY_CACHE_DIR: ${{ github.workspace }}/.cache/trivy' <<<"$reuse_block" \
  && ok "the reuse step reads the downloaded vulnerability DB" \
  || bad "the reuse step does not point trivy at the downloaded DB"
script_platform="$(sed -n 's/.*opensandbox-server-image\.sh "\$OPENSANDBOX_IMAGE" [^ ]* \([^ ]*\) .*/\1/p' "$WORKFLOW")"
build_block="$(awk '/- id: build_opensandbox_fresh/{take=1} take{print} take && /tags:/{exit}' "$WORKFLOW")"
grep -qF "platforms: $script_platform" <<<"$build_block" \
  && ok "the keyed platform is the built platform ($script_platform)" \
  || bad "the platform keyed ($script_platform) is not the one built"
cosign_line="$(grep -n 'sigstore/cosign-installer' "$WORKFLOW" | head -1 | cut -d: -f1)"
[ -n "$cosign_line" ] && [ -n "$reuse_line" ] && [ "$cosign_line" -lt "$reuse_line" ] \
  && ok "cosign is installed before the reuse step verifies" \
  || bad "cosign is installed after the reuse step"
grep -A1 -F -- '- id: build_opensandbox_fresh' "$WORKFLOW" | grep -qF "if: steps.opensandbox_reuse.outputs.digest == ''" \
  && ok "deploy.yml builds only when nothing is reused" \
  || bad "the server image build is not conditional on the reuse step"
grep -qF '${{ env.OPENSANDBOX_IMAGE }}:${{ steps.opensandbox_reuse.outputs.tag }}' "$WORKFLOW" \
  && ok "a fresh build is published under the inputs tag" \
  || bad "a fresh build is not tagged for reuse"
grep -qF 'digest="${REUSED:-$BUILT}"' "$WORKFLOW" \
  && ok "later steps read one digest, reused or built" \
  || bad "deploy.yml lost the step that merges the reused and the built digest"
script_version="$(sed -n 's/.*opensandbox-server-image\.sh "\$OPENSANDBOX_IMAGE" \([^ ]*\) .*/\1/p' "$WORKFLOW")"
grep -qF "build-args: OPENSANDBOX_SERVER_VERSION=$script_version" "$WORKFLOW" \
  && ok "the keyed version is the built version ($script_version)" \
  || bad "the version keyed ($script_version) is not the build-arg"
db_line="$(grep -n 'trivy image --download-db-only' "$WORKFLOW" | head -1 | cut -d: -f1)"
reuse_line="$(grep -n -- '- id: opensandbox_reuse' "$WORKFLOW" | head -1 | cut -d: -f1)"
[ -n "$db_line" ] && [ -n "$reuse_line" ] && [ "$db_line" -lt "$reuse_line" ] \
  && ok "the vulnerability DB is downloaded before the reuse scan" \
  || bad "the reuse step runs before the vulnerability DB is downloaded"

[ "$FAILURES" -eq 0 ] || { echo "$FAILURES failure(s)" >&2; exit 1; }
echo "all opensandbox-server-image tests passed"
