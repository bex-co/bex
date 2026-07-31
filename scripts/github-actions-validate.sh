#!/usr/bin/env bash
# Keep every third-party action invocation on the reviewed Node 24-compatible
# major. Adding or upgrading an action requires checking its upstream action.yml,
# release notes, inputs, permissions, and nested composite actions first.
set -euo pipefail
cd "$(dirname "$0")/.."

expected_actions='actions/cache@v6
actions/checkout@v7
actions/setup-go@v7
actions/setup-node@v7
anchore/sbom-action/download-syft@v0
aquasecurity/trivy-action@v0.36.0
azure/setup-helm@v5
azure/setup-kubectl@v5
docker/build-push-action@v7
docker/login-action@v4
docker/setup-buildx-action@v4
hashicorp/setup-packer@v3
hashicorp/setup-terraform@v4
sigstore/cosign-installer@v3'

actual_actions="$({
  grep -rhoE 'uses:[[:space:]]+[^[:space:]#]+' .github/workflows/*.yml || true
} | awk '{print $2}' | grep -v '^\./' | sort -u)"

if ! diff -u <(printf '%s\n' "$expected_actions") <(printf '%s\n' "$actual_actions"); then
  echo "FAIL: GitHub Action refs differ from the reviewed Node 24-compatible inventory" >&2
  exit 1
fi

if grep -REn "node-version:[[:space:]]*['\"]?20(['\"]|$)" .github/workflows; then
  echo "FAIL: workflows must not install the end-of-life Node 20 runtime" >&2
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

echo "PASS: GitHub Action refs, explicit Node runtimes, Gitleaks scan, and supersession wiring are current"
