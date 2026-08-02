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
seed deploy/gitops/base/bex.yaml $'images:\n  - controller=ghcr.io/x@sha256:'"$(printf 'a%.0s' {1..64})" "base bex.yaml"
seed deploy/gitops/base/values/opensandbox-controller.values.yaml \
  $'controller:\n  image:\n    repository: ghcr.io/x/controller\n    tag: v0.2.0-bex-snapjobns@sha256:'"$(printf 'a%.0s' {1..64})" \
  "base OpenSandbox controller values"
git push -q origin main
BASE="$(git rev-parse HEAD)"

# assert <label> <expected-rc> — reset origin to BASE, run the case body (which
# advances origin/main), then run the script with SHA=BASE and check its rc.
run_case() {
  local label="$1" want="$2"; shift 2
  git push -qf origin "$BASE":main # reset origin/main to the base
  "$@"                              # case body advances origin/main (or not)
  set +e
  "$SCRIPT" "$BASE" >/dev/null 2>&1
  local got=$?
  set -e
  if [ "$got" -ne "$want" ]; then
    echo "FAIL: $label — exit $got, want $want" >&2
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

# current: origin/main unchanged from BASE → exit 1
run_case "current (no drift)" 1 true
# superseded by a lego/ change → exit 0
run_case "superseded: lego change" 0 advance lego/app/main.go "package app // v2"
# superseded by a deploy.yml change → exit 0
run_case "superseded: deploy.yml change" 0 advance .github/workflows/deploy.yml "name: deploy v2"
# a preceding run's generated digest write-back (bex.yaml digest only) → exit 1
run_case "generated digest only (excluded)" 1 advance deploy/gitops/base/bex.yaml \
  $'images:\n  - controller=ghcr.io/x@sha256:'"$(printf 'b%.0s' {1..64})"
# the patched controller's generated digest write-back is excluded too
run_case "generated controller digest only (excluded)" 1 advance \
  deploy/gitops/base/values/opensandbox-controller.values.yaml \
  $'controller:\n  image:\n    repository: ghcr.io/x/controller\n    tag: v0.2.0-bex-snapjobns@sha256:'"$(printf 'b%.0s' {1..64})"
# a non-deploy-triggering change (docs) → exit 1
run_case "non-triggering (docs)" 1 advance docs/notes.md "hello"

if [ "$fails" -eq 0 ]; then
  echo "PASS: deploy-superseded.sh"
else
  echo "FAIL: $fails case(s)" >&2
  exit 1
fi
