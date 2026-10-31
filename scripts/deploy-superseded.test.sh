#!/usr/bin/env bash
# Unit test for scripts/deploy-superseded.sh (w1/m59): pins the supersession
# path-filter/diff logic against a throwaway git repo, so a drift that would
# either pin stale images or falsely skip a build fails CI, not prod.
set -euo pipefail

SCRIPT="$(cd "$(dirname "$0")" && pwd)/deploy-superseded.sh"
[ -x "$SCRIPT" ] || { echo "FAIL: $SCRIPT not executable" >&2; exit 1; }

fails=0
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

# A bare "origin" plus a working clone, isolated from the real repo/user config.
export GIT_CONFIG_GLOBAL="$tmp/gitconfig" GIT_CONFIG_SYSTEM=/dev/null
git config --global user.email t@t && git config --global user.name t
git config --global init.defaultBranch main
git init --bare -q "$tmp/origin.git"
git clone -q "$tmp/origin.git" "$tmp/wt"
cd "$tmp/wt"

seed() { # create a file at $1 with content $2, committing under a fresh dir tree
  mkdir -p "$(dirname "$1")"
  printf '%s\n' "$2" >"$1"
  git add -A && git commit -qm "$3"
}

# Base commit carries one file in a filtered path; push it as origin/main.
seed lego/app/main.go "package app // v1" "base"
seed lego/cli/main.go "package main // CLI v1" "base CLI"
seed deploy/gitops/base/bex.yaml $'images:\n  - controller=ghcr.io/x@sha256:'"$(printf 'a%.0s' {1..64})" "base bex.yaml"
seed deploy/gitops/base/dashboard.yaml $'images:\n  - dashboard=ghcr.io/x@sha256:'"$(printf 'a%.0s' {1..64})" "base dashboard.yaml"
seed deploy/opensandbox/kustomization.yaml \
  $'images:\n  - name: opensandbox-server\n    newName: ghcr.io/x\n    digest: sha256:'"$(printf 'a%.0s' {1..64})" \
  "base opensandbox kustomization"
seed deploy/gitops/base/values/opensandbox-controller.values.yaml \
  $'controller:\n  image:\n    repository: ghcr.io/x/controller\n    tag: v0.2.0-bex-snapjobns-terminalpod@sha256:'"$(printf 'a%.0s' {1..64})" \
  "base OpenSandbox controller values"
seed lego/operator/config/api/deployment.yaml \
  "  value: ghcr.io/bex-co/bex-agent-sandbox@sha256:$(printf 'c%.0s' {1..64})" \
  "base agent image"
git push -q origin main
BASE="$(git rev-parse HEAD)"

# assert <label> <expected-rc> — reset origin to BASE, run the case body (which
# advances origin/main), then run the script with SHA=BASE and check its rc.
run_case() {
  local label="$1" want="$2"; shift 2
  git push -qf origin "$BASE":main # reset origin/main to the base
  "$@"                              # case body advances origin/main (or not)
  local output
  set +e
  output="$("$SCRIPT" "$BASE" 2>&1)"
  local got=$?
  set -e
  if [ "$got" -ne "$want" ]; then
    echo "FAIL: $label — exit $got, want $want" >&2
    [ -z "$output" ] || echo "$output" >&2
    fails=$((fails + 1))
  else
    echo "ok: $label (exit $got)"
  fi
}

advance() { # commit $2 at path $1 on top of origin/main and push
  git fetch -q origin main && git reset -q --hard origin/main
  seed "$1" "$2" "advance $1"
  git push -q origin main
}

advance_with_source() {
  advance lego/backend/cmd/api/main.go "package main // newer source"
  advance "$@"
}

advance_with_cli() {
  advance lego/cli/internal/launch/launch.go "package launch // CLI update"
  advance "$@"
}

rename_input() {
  git fetch -q origin main && git reset -q --hard origin/main
  mkdir -p "$(dirname "$2")"
  git mv "$1" "$2"
  git commit -qm "rename $1 to $2"
  git push -q origin main
}

remove_cli() {
  git fetch -q origin main && git reset -q --hard origin/main
  git rm -q lego/cli/main.go
  git commit -qm "remove CLI file"
  git push -q origin main
}

# current: origin/main unchanged from BASE → exit 1
run_case "current (no drift)" 1 true
# CLI releases do not schedule a replacement platform deploy.
run_case "CLI-only root file (excluded)" 1 advance lego/cli/go.mod "module cli"
run_case "CLI-only nested file (excluded)" 1 advance lego/cli/internal/launch/launch.go "package launch"
run_case "CLI-only deletion (excluded)" 1 remove_cli
run_case "platform file renamed into CLI" 3 rename_input lego/app/main.go lego/cli/internal/moved/main.go
run_case "CLI file renamed into platform" 3 rename_input lego/cli/main.go lego/backend/cmd/moved/main.go
run_case "CLI plus backend change" 3 advance_with_cli lego/backend/cmd/api/main.go "package main"
run_case "CLI plus dashboard change" 3 advance_with_cli dashboard/src/routes/index.tsx "export default function Home() {}"
# newer image source only → exit 3: a newer run is queued, and these images
# are safe to pin onto the unchanged manifests (w1/116)
run_case "newer source: lego change" 3 advance lego/app/main.go "package app // v2"
run_case "newer source: dashboard change" 3 advance dashboard/src/app.tsx "export {}"
# Newer manifests are already live through Argo on the older pinned images, so
# these images narrow the gap: pin them (exit 4) rather than starve (w4/439).
run_case "newer manifests: operator config (CRD) change" 4 advance lego/operator/config/crd/bases/apps.yaml "kind: CustomResourceDefinition"
run_case "newer manifests: gitops manifest change" 4 advance deploy/gitops/base/other.yaml "kind: ConfigMap"
run_case "newer manifests: manifest plus source change" 4 advance_with_source deploy/opensandbox/extra.yaml "kind: ConfigMap"
run_case "newer manifests: deploy.yml change" 4 advance .github/workflows/deploy.yml "name: deploy v2"
# a preceding run's generated digest write-back (bex.yaml digest only) → exit 1
run_case "generated digest only (excluded)" 1 advance deploy/gitops/base/bex.yaml \
  $'images:\n  - controller=ghcr.io/x@sha256:'"$(printf 'b%.0s' {1..64})"
run_case "CLI plus generated platform digest only (excluded)" 1 advance_with_cli deploy/gitops/base/bex.yaml \
  $'images:\n  - controller=ghcr.io/x@sha256:'"$(printf 'b%.0s' {1..64})"
# the patched controller's generated digest write-back is excluded too
run_case "generated controller digest only (excluded)" 1 advance \
  deploy/gitops/base/values/opensandbox-controller.values.yaml \
  $'controller:\n  image:\n    repository: ghcr.io/x/controller\n    tag: v0.2.0-bex-snapjobns-terminalpod@sha256:'"$(printf 'b%.0s' {1..64})"
run_case "controller tag change is substantive" 0 advance \
  deploy/gitops/base/values/opensandbox-controller.values.yaml \
  $'controller:\n  image:\n    repository: ghcr.io/x/controller\n    tag: v0.2.0-bex-next@sha256:'"$(printf 'b%.0s' {1..64})"
run_case "generated agent digest only (excluded)" 1 advance lego/operator/config/api/deployment.yaml \
  "  value: ghcr.io/bex-co/bex-agent-sandbox@sha256:$(printf 'd%.0s' {1..64})"
run_case "CLI plus generated agent digest only (excluded)" 1 advance_with_cli lego/operator/config/api/deployment.yaml \
  "  value: ghcr.io/bex-co/bex-agent-sandbox@sha256:$(printf 'd%.0s' {1..64})"
run_case "agent manifest drift beyond digest" 0 advance lego/operator/config/api/deployment.yaml \
  $'replicas: 2\n  value: ghcr.io/bex-co/bex-agent-sandbox@sha256:'"$(printf 'd%.0s' {1..64})"
run_case "CLI plus substantive agent manifest drift" 0 advance_with_cli lego/operator/config/api/deployment.yaml \
  $'replicas: 2\n  value: ghcr.io/bex-co/bex-agent-sandbox@sha256:'"$(printf 'd%.0s' {1..64})"
# a non-deploy-triggering change (docs) → exit 1
run_case "non-triggering (docs)" 1 advance docs/notes.md "hello"

pin() { # the write-back's commit pinning producer $1, on top of origin/main
  git fetch -q origin main && git reset -q --hard origin/main
  printf 'images:\n  - controller=ghcr.io/x@sha256:%s\n' "$(printf '%s' "$1" | sha256sum | cut -c1-64)" >deploy/gitops/base/bex.yaml
  git add -A && git commit -qm "chore(deploy): pin platform images to ${1::12} [skip ci]"
  git push -q origin main
}

# A re-run of an old run must never roll production back over a newer pin.
pin_newer_producer() {
  advance lego/app/main.go "package app // newer"
  pin "$(git rev-parse HEAD)"
  advance deploy/gitops/base/other.yaml "kind: ConfigMap # newer"
}
run_case "older than the current pin" 0 pin_newer_producer
pin_this_producer() {
  pin "$BASE"
  advance deploy/gitops/base/other.yaml "kind: ConfigMap # after pin"
}
run_case "newer manifests after this commit's own pin" 4 pin_this_producer

# w4/439: under steady manifest pushes, each finished run still pins. Every
# run's build overlaps a manifest commit and a source commit; each must pin, so
# production trails main by at most one run.
git push -qf origin "$BASE":main
producer="$BASE"
for round in 1 2 3; do
  advance deploy/gitops/base/other.yaml "kind: ConfigMap # round $round"
  advance lego/app/main.go "package app // round $round"
  next="$(git rev-parse HEAD)"
  set +e
  "$SCRIPT" "$producer" >/dev/null 2>&1
  got=$?
  set -e
  if [ "$got" -ne 4 ]; then
    echo "FAIL: steady manifest pushes round $round — exit $got, want 4 (pin)" >&2
    fails=$((fails + 1))
  fi
  pin "$producer"
  producer="$next"
done
echo "ok: steady manifest pushes pin every run"

# w2/050: the message names the commit that moved the manifests, not a newer
# docs-only tip that merely sits on top of it.
git push -qf origin "$BASE":main
advance lego/operator/config/crd/bases/apps.yaml "kind: CustomResourceDefinition # v2"
manifest_commit="$(git rev-parse --short HEAD)"
advance docs/notes.md "docs on top"
docs_tip="$(git rev-parse --short HEAD)"
set +e
named="$("$SCRIPT" "$BASE" 2>&1)"
set -e
if [[ "$named" == *"changed by: $manifest_commit"* && "$named" != *"$docs_tip"* ]]; then
  echo "ok: superseded message names the manifest commit, not the docs tip"
else
  echo "FAIL: superseded message should name $manifest_commit and not $docs_tip, got: $named" >&2
  fails=$((fails + 1))
fi

if [ "$fails" -eq 0 ]; then
  echo "PASS: deploy-superseded.sh"
else
  echo "FAIL: $fails case(s)" >&2
  exit 1
fi
