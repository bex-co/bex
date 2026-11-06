#!/usr/bin/env bash
# Unit test for scripts/deploy-pin-stale.sh (w9/m173) against a throwaway repo.
set -euo pipefail

SCRIPT="$(cd "$(dirname "$0")" && pwd)/deploy-pin-stale.sh"
[ -x "$SCRIPT" ] || { echo "FAIL: $SCRIPT not executable" >&2; exit 1; }

fails=0
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
export GIT_CONFIG_GLOBAL="$tmp/gitconfig" GIT_CONFIG_SYSTEM=/dev/null
git config --global user.email t@t && git config --global user.name t
git config --global init.defaultBranch main
git init --bare -q "$tmp/origin.git"
git clone -q "$tmp/origin.git" "$tmp/wt"
cd "$tmp/wt"

commit_at() { # commit_at <unix-time> <subject>
  GIT_COMMITTER_DATE="@$1 +0000" GIT_AUTHOR_DATE="@$1 +0000" \
    git commit -q --allow-empty -m "$2"
}

check() { # check <label> <want-rc> <now> <max>
  local got
  set +e
  DEPLOY_PIN_STALE_NOW="$3" "$SCRIPT" "$4" >/dev/null 2>&1
  got=$?
  set -e
  if [ "$got" -ne "$2" ]; then
    echo "FAIL: $1 — exit $got, want $2" >&2
    fails=$((fails + 1))
  fi
}

commit_at 1000 "feat: base"
git push -q origin main
check "no pin commit reads as stale" 0 2000 7200

commit_at 10000 "chore(deploy): pin platform images to 0123456789ab [skip ci]"
commit_at 12000 "fix: a later manifest commit"
git push -q origin main
check "a pin younger than the bound is fresh" 1 15000 7200
check "a pin at exactly the bound is stale" 0 17200 7200
check "a pin older than the bound is stale" 0 30000 7200
check "a non-numeric bound is an error" 2 15000 2h

commit_at 40000 "docs: mentions chore(deploy): pin platform images to 0123456789ab in the body only"
git push -q origin main
check "only a pin subject counts, so the 10000 pin is the latest" 0 30000 7200

if [ "$fails" -ne 0 ]; then
  echo "deploy-pin-stale.test.sh: $fails failure(s)" >&2
  exit 1
fi
echo "PASS: deploy-pin-stale.sh"
