#!/usr/bin/env bash
# Supply-chain + runtime guard for bex's GitHub Actions (w7/m65, m1/m59, m62):
#
#  1. Every third-party `uses:` ref is pinned to a full 40-hex commit SHA, so a
#     retagged/compromised upstream release (the 2025 tj-actions/changed-files
#     attack vector) can never change what runs in CI. `.github/dependabot.yml`
#     keeps the pins fresh; adding/upgrading an action requires reviewing its
#     upstream action.yml, release notes, inputs, permissions, and nested
#     composite actions first, then updating the pinned SHA + `# vX.Y.Z` comment.
#  2. The reviewed inventory (Node 24-compatible majors) is diffed against the
#     tree so a new/changed action can't slip in unreviewed.
#  3. No end-of-life Node 20 runtime; deploy.yml keeps its checksum-pinned
#     Gitleaks scan and its w1/m59 supersession wiring.
#
# Setting WORKFLOWS_DIR overrides the scanned tree; the self-test
# (github-actions-validate.test.sh) points it at fixtures to exercise the
# portable checks (SHA pin, Node 20 absence) in isolation. Only the default,
# override-free invocation additionally runs the canonical-tree checks (reviewed
# inventory diff + deploy.yml content), which pin this repo's specific tree.
set -euo pipefail
cd "$(dirname "$0")/.."

# The seam is "was an override supplied?", not a path string-compare — so an
# explicit WORKFLOWS_DIR=.github/workflows (or a differently-spelled path to the
# same tree) can't silently drop the canonical checks.
if [ -n "${WORKFLOWS_DIR:-}" ]; then
  canonical_tree=0
else
  canonical_tree=1
  WORKFLOWS_DIR=".github/workflows"
fi

# Third-party `uses:` refs, comment-stripped, with fully-commented lines and the
# 3 local `./` reusable-workflow refs excluded (path refs to this repo, exempt).
third_party_refs() {
  grep -vhE '^[[:space:]]*#' "$WORKFLOWS_DIR"/*.yml 2>/dev/null \
    | grep -oE 'uses:[[:space:]]+[^[:space:]#]+' \
    | awk '{print $2}' | grep -v '^\./' | sort -u
}

# 1. SHA-pin enforcement: every third-party ref must be a full 40-hex commit SHA.
unpinned="$(third_party_refs | grep -vE '@[0-9a-f]{40}$' || true)"
if [ -n "$unpinned" ]; then
  echo "FAIL: third-party action refs must be pinned to a full 40-hex commit SHA (not a mutable tag):" >&2
  printf '  %s\n' $unpinned >&2
  exit 1
fi

# 3a. No end-of-life Node 20 runtime.
if grep -REn "node-version:[[:space:]]*['\"]?20(['\"]|$)" "$WORKFLOWS_DIR"; then
  echo "FAIL: workflows must not install the end-of-life Node 20 runtime" >&2
  exit 1
fi

# 4. Host-key pin coverage (w1/m68 F3). scripts/lib/ssh-hostkey.sh falls back to
# StrictHostKeyChecking=accept-new when no pin is supplied, so a workflow that
# fetches /etc/kubernetes/admin.conf over SSH without wiring BEX_SSH_KNOWN_HOSTS
# trusts a first-seen control-plane key and hands every later step whatever
# kubeconfig that host returned. w1/m66 wired two of the three such workflows and
# missed openbao-restore-drill.yml — the one that then uses the OpenBao unseal
# keys and root token. This derives the list from the tree rather than naming
# workflows, so a NEW admin.conf fetcher is caught the day it is added.
admin_conf_fetchers() {
  grep -lE 'scripts/(fetch-app-kubeconfig|verify-substrate)\.sh' "$WORKFLOWS_DIR"/*.yml 2>/dev/null || true
}
unpinned_fetchers=""
for wf in $(admin_conf_fetchers); do
  grep -Fq 'BEX_SSH_KNOWN_HOSTS' "$wf" || unpinned_fetchers="$unpinned_fetchers $wf"
done
if [ -n "$unpinned_fetchers" ]; then
  echo "FAIL: these workflows fetch admin.conf over SSH without wiring the BEX_SSH_KNOWN_HOSTS pin," >&2
  echo "      so they trust the control-plane host key on first use:" >&2
  printf '  %s\n' $unpinned_fetchers >&2
  exit 1
fi

# The remaining checks pin the canonical tree's reviewed inventory + deploy.yml
# wiring; they don't apply to a fixture dir, so stop here under an override.
if [ "$canonical_tree" -eq 0 ]; then
  echo "PASS: third-party refs SHA-pinned and Node 20 absent (fixture: $WORKFLOWS_DIR)"
  exit 0
fi

# 2. Reviewed inventory (SHA-pinned; sorted to match third_party_refs). Keep in
# lockstep with the workflow tree — a Dependabot actions bump updates both.
expected_actions='actions/cache@55cc8345863c7cc4c66a329aec7e433d2d1c52a9
actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1
actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e
actions/setup-node@820762786026740c76f36085b0efc47a31fe5020
anchore/sbom-action/download-syft@e22c389904149dbc22b58101806040fa8d37a610
aquasecurity/trivy-action@ed142fd0673e97e23eac54620cfb913e5ce36c25
azure/setup-helm@9bc31f4ebc9c6b171d7bfbaa5d006ae7abdb4310
azure/setup-kubectl@829323503d1be3d00ca8346e5391ca0b07a9ab0d
docker/build-push-action@53b7df96c91f9c12dcc8a07bcb9ccacbed38856a
docker/login-action@dbcb813823bdd20940b903addbd779551569679f
docker/setup-buildx-action@bb05f3f5519dd87d3ba754cc423b652a5edd6d2c
hashicorp/setup-packer@ce93c3c08a6c2ff2275bf4b54ff0d9a75f6c9789
hashicorp/setup-terraform@dfe3c3f87815947d99a8997f908cb6525fc44e9e
sigstore/cosign-installer@6f9f17788090df1f26f669e9d70d6ae9567deba6'

if ! diff -u <(printf '%s\n' "$expected_actions") <(third_party_refs); then
  echo "FAIL: GitHub Action refs differ from the reviewed SHA-pinned inventory" >&2
  exit 1
fi

if ! grep -Fq 'GITLEAKS_VERSION: 8.30.1' .github/workflows/deploy.yml \
  || ! grep -Fq 'GITLEAKS_LINUX_X64_SHA256: 551f6fc83ea457d62a0d98237cbad105af8d557003051f41f3e7ca7b3f2470eb' .github/workflows/deploy.yml \
  || ! grep -Fq 'gitleaks git --no-banner --redact --exit-code 1 --log-opts="$log_opts" .' .github/workflows/deploy.yml; then
  echo "FAIL: deploy must checksum-pin and execute the reviewed Gitleaks CLI scanner" >&2
  exit 1
fi

# w1/m59: a superseded deploy run must be skipped/neutralized (never red at the
# write-back), and no image built when superseded pre-build. Pin the wiring so it
# can't silently drift back to the old exit-1 path.
if ! grep -Eq '^  check-supersession:' .github/workflows/deploy.yml \
  || ! grep -Fq "needs.check-supersession.outputs.superseded != 'true'" .github/workflows/deploy.yml \
  || [ "$(grep -cF 'bash scripts/deploy-superseded.sh "$GITHUB_SHA"' .github/workflows/deploy.yml)" -lt 2 ]; then
  echo "FAIL: deploy.yml lost the w1/m59 supersession pre-check / write-back wiring (scripts/deploy-superseded.sh)" >&2
  exit 1
fi
if [ "$(grep -cF "env.SUPERSEDED != 'true'" .github/workflows/deploy.yml)" -lt 4 ]; then
  echo "FAIL: deploy.yml rollout steps must gate on env.SUPERSEDED so a mid-build supersession concludes neutral (w1/m59)" >&2
  exit 1
fi

# Anti-vacuity for check 4: the pin-coverage loop passes trivially over a tree
# with no admin.conf fetchers, so pin the known three (deploy, app-cluster,
# openbao-restore-drill). Removing a fetcher is fine; silently having none is not.
if [ "$(admin_conf_fetchers | wc -l | tr -d ' ')" -lt 3 ]; then
  echo "FAIL: expected at least 3 workflows fetching admin.conf over SSH — the pin-coverage check (4) has gone vacuous" >&2
  exit 1
fi

echo "PASS: third-party actions SHA-pinned, reviewed inventory current, Node 20 absent, Gitleaks + supersession wiring intact, admin.conf fetchers host-key pinned"
