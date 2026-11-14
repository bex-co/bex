#!/usr/bin/env bash
# ship-gates.test.sh — ship-gates.sh's red-main report against a stubbed `gh`
# that prints raw run JSON, so the gate's own selection of the newest
# completed, non-cancelled run is what is tested (no network). A red main is a
# warning, never a stop (user decision 2026-10-21). The base ref is HEAD, so
# the backend-suite gate has no changes to test and does not run.
set -euo pipefail

root=$(cd "$(dirname "$0")/.." && pwd)
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

fail() {
  echo "FAIL: $*" >&2
  exit 1
}
stub() { # the JSON `gh run list --json conclusion,status,headSha,headBranch,url,createdAt` prints
  printf '%s\n' "$1" >"$tmp/runs.json"
  printf '#!/bin/sh\ncat "%s"\n' "$tmp/runs.json" >"$tmp/gh"
  chmod +x "$tmp/gh"
}
run() { PATH="$tmp:$PATH" bash "$root/scripts/ship-gates.sh" "$@" 2>"$tmp/err"; }
run_ok() { run "$@" || fail "$1: expected the ship to proceed: $(cat "$tmp/err")"; }
# The ship proceeds and the red run is named, so the caller can fix CI first.
run_warned() {
  run "$@" || fail "$1: a red main must never stop the ship: $(cat "$tmp/err")"
  grep -q "warning: main is red" "$tmp/err" || fail "red-main warning missing: $(cat "$tmp/err")"
}
run_quiet() {
  run_ok "$@"
  if grep -q "main is red" "$tmp/err"; then fail "$1: a green main reported red: $(cat "$tmp/err")"; fi
}

# One run as `gh run list` prints it: <conclusion> <status> <hour> [branch]
# [event]. The hour is its createdAt, the only thing that orders runs.
r() {
  printf '{"conclusion":"%s","status":"%s","headSha":"%02dabc123def4","headBranch":"%s","event":"%s","url":"https://example.test/runs/%d","createdAt":"2026-10-05T%02d:00:00Z"}' \
    "$1" "$2" "$3" "${4:-main}" "${5:-push}" "$3" "$3"
}

stub "[$(r failure completed 10)]"
run_warned HEAD
stub "[$(r timed_out completed 10)]"
run_warned HEAD
stub "[$(r startup_failure completed 10)]"
run_warned HEAD
stub "[$(r failure completed 10)]"
run_warned HEAD --fixes-red-main
grep -q "marked as the fix" "$tmp/err" || fail "--fixes-red-main not acknowledged: $(cat "$tmp/err")"
run_warned --fixes-red-main HEAD

# A run still in progress or a cancelled one is not evidence of health: look
# past it to the newest real verdict.
stub "[$(r "" in_progress 12),$(r failure completed 11),$(r success completed 10)]"
run_warned HEAD
stub "[$(r cancelled completed 12),$(r failure completed 11)]"
run_warned HEAD
stub "[$(r skipped completed 13),$(r neutral completed 12),$(r failure completed 11)]"
run_warned HEAD
stub "[$(r "" in_progress 13),$(r cancelled completed 12),$(r success completed 11),$(r failure completed 10)]"
run_quiet HEAD

# Whatever the listing order, the newest run decides.
stub "[$(r failure completed 10),$(r success completed 11)]"
run_quiet HEAD
stub "[$(r success completed 10),$(r failure completed 11)]"
run_warned HEAD

# Only main's runs judge main: a manual dispatch elsewhere does not.
stub "[$(r failure completed 12 feature),$(r success completed 11)]"
run_quiet HEAD
stub "[$(r success completed 12 feature),$(r failure completed 11)]"
run_warned HEAD
grep -q "(created 2026-10-05T11:00:00Z)" "$tmp/err" || fail "the warning names no run date: $(cat "$tmp/err")"

stub "[]"
run_ok HEAD
grep -q "warning" "$tmp/err" || fail "no completed run gave no warning"

# The suites run on every main push in their own workflows: a red suite is a
# red main even while the batched deploy runs conclude success, and a suite's
# pull-request run is not main's verdict.
printf '[%s]\n' "$(r success completed 10)" >"$tmp/green.json"
printf '[%s]\n' "$(r failure completed 11)" >"$tmp/red.json"
printf '[%s,%s]\n' "$(r failure completed 12 main pull_request)" "$(r success completed 10)" >"$tmp/pr-red.json"
suite_stub() { # <file for backend-test.yml runs>
  printf '#!/bin/sh\ncase "$*" in *backend-test.yml*) cat "%s" ;; *) cat "%s" ;; esac\n' "$1" "$tmp/green.json" >"$tmp/gh"
  chmod +x "$tmp/gh"
}
suite_stub "$tmp/red.json"
run_warned HEAD
grep -q "backend-test.yml run for 11abc123d" "$tmp/err" || fail "the warning does not name the red suite: $(cat "$tmp/err")"
suite_stub "$tmp/pr-red.json"
run_quiet HEAD

printf '#!/bin/sh\nexit 1\n' >"$tmp/gh"
run_ok HEAD
grep -q "warning" "$tmp/err" || fail "unreadable status gave no warning"

# Repository security drift (w3/m365) is a warning, never a stop: a gh that
# answers as an admin, with push protection set by $PP. GITHUB_REPOSITORY is
# pinned because CI always sets it to the real repository.
cat >"$tmp/gh" <<STUB
#!/bin/sh
case "\$*" in
  "run list"*) printf '%s\n' '[$(r success completed 10)]' ;;
  "repo view"*) echo o/r ;;
  "api repos/o/r") printf '{"security_and_analysis":{"secret_scanning":{"status":"enabled"},"secret_scanning_push_protection":{"status":"%s"}}}\n' "\$PP" ;;
  "api repos/o/r/rules/branches/main") echo '[{"type":"deletion","ruleset_id":7},{"type":"non_fast_forward","ruleset_id":7}]' ;;
  "api repos/o/r/rulesets/7") echo '{"id":7,"bypass_actors":[]}' ;;
  *) exit 1 ;;
esac
STUB
chmod +x "$tmp/gh"
GITHUB_REPOSITORY=o/r PP=disabled run_quiet HEAD
grep -q "secret scanning or main's history protection drifted" "$tmp/err" || fail "push protection off gave no warning: $(cat "$tmp/err")"
grep -q "DRIFT: secret_scanning_push_protection is disabled" "$tmp/err" || fail "the warning does not name the setting: $(cat "$tmp/err")"
GITHUB_REPOSITORY=o/r PP=enabled run_quiet HEAD
if grep -q "drifted" "$tmp/err"; then fail "matching settings warned: $(cat "$tmp/err")"; fi

if bash "$root/scripts/ship-gates.sh" 2>/dev/null; then fail "a missing base ref must be refused"; fi

# The dashboard gate (w6/m159), in a throwaway repo with a stubbed yarn: a
# dashboard change runs lint and test from dashboard/, a failing check blocks,
# and a push without dashboard changes runs nothing.
repo="$tmp/repo"
mkdir -p "$repo/scripts" "$repo/dashboard" "$repo/docs" "$repo/.github/scripts" "$repo/.github/workflows" "$repo/lego/operator"
cp "$root/scripts/ship-gates.sh" "$root/scripts/repo-artifact-guard.sh" "$root/scripts/ci_test_inputs.py" "$repo/scripts/"
cp "$root/.github/scripts/native-common-qualify.py" "$repo/.github/scripts/"
# Stands in for the native custody suites: logs that it ran, fails on demand.
cat >"$repo/.github/scripts/test_native_oci_stub.py" <<PY
import os, unittest
class Custody(unittest.TestCase):
    def test_custody(self):
        open("$tmp/native.log", "a").write("ran\\n")
        self.assertFalse(os.environ.get("NATIVE_FAIL"))
PY
git -C "$repo" init -q -b main
commit() { git -C "$repo" add -A && git -C "$repo" -c user.name=t -c user.email=t@t -c core.hooksPath=/dev/null -c commit.gpgsign=false commit -qm "$1"; }
echo '{}' >"$repo/dashboard/package.json"
commit base
stub "[$(r success completed 10)]"
cat >"$tmp/yarn" <<YARN
#!/bin/sh
echo "\$(basename "\$PWD") \$*" >>"$tmp/yarn.log"
[ "\$1" = "\${YARN_FAIL:-}" ] && exit 1
# A file the tests would trip over: only the committed tree counts.
[ ! -e BROKEN ] || exit 1
exit 0
YARN
chmod +x "$tmp/yarn"
gate() { (cd "$repo" && BEX_SHIP_GATES_TREE="$tmp/tree" PATH="$tmp:$PATH" bash scripts/ship-gates.sh "$@") 2>"$tmp/err"; }
echo 'docs' >"$repo/docs/a.md"
commit docs
rm -f "$tmp/yarn.log"
gate HEAD~1 || fail "a docs-only push failed the gate: $(cat "$tmp/err")"
[ ! -e "$tmp/yarn.log" ] || fail "a push without dashboard changes ran yarn: $(cat "$tmp/yarn.log")"
echo 'export {}' >"$repo/dashboard/x.ts"
commit dash
rm -f "$tmp/yarn.log"
gate HEAD~1 || fail "a passing dashboard push failed the gate: $(cat "$tmp/err")"
if ! { grep -qx "dashboard install --immutable" "$tmp/yarn.log" && grep -qx "dashboard lint" "$tmp/yarn.log" && grep -qx "dashboard test" "$tmp/yarn.log"; }; then
  fail "the dashboard gate did not install, lint and test in dashboard/: $(cat "$tmp/yarn.log")"
fi
rm -f "$tmp/yarn.log"
if YARN_FAIL="test" gate HEAD~1; then fail "a failing dashboard test did not block the push"; fi
grep -qx "dashboard test" "$tmp/yarn.log" || fail "the push was blocked before the dashboard tests ran: $(cat "$tmp/err")"
grep -q "ship-gates: ok" "$tmp/err" && fail "a failing dashboard test still reported ok"
rm -f "$tmp/yarn.log"
if YARN_FAIL="install" gate HEAD~1; then fail "a failing dashboard install did not block the push"; fi
grep -qx "dashboard lint" "$tmp/yarn.log" && fail "lint ran after a failed install"

# The gates test HEAD in a worktree, never the working tree: an uncommitted
# file neither breaks a good commit nor rescues a broken one.
grep -q "testing $(git -C "$repo" rev-parse --short HEAD) in $tmp/tree" "$tmp/err" || fail "the gate did not test HEAD in its worktree: $(cat "$tmp/err")"
touch "$repo/dashboard/BROKEN"
gate HEAD~1 || fail "an uncommitted file failed a good commit: $(cat "$tmp/err")"
rm "$repo/dashboard/BROKEN"
echo broken >"$repo/dashboard/BROKEN"
commit broken
if gate HEAD~1; then fail "a broken commit passed the gate"; fi
git -C "$repo" rm -q dashboard/BROKEN
commit unbroken
[ ! -e "$tmp/tree.lock" ] || fail "the gate left its worktree locked"


# The native custody gate (w6/107): a custody-pinned file, a native workflow or
# a native module runs the custody suites, a failure blocks, and anything else
# runs nothing.
native_gate() { # <path> <expect: ran|skipped> [NATIVE_FAIL]
  echo "$RANDOM" >>"$repo/$1"
  commit "$1"
  rm -f "$tmp/native.log"
  if NATIVE_FAIL="${3:-}" gate HEAD~1; then
    [ -z "${3:-}" ] || fail "$1: a failing custody suite did not block the push"
  else
    [ -n "${3:-}" ] || fail "$1: a passing push failed the gate: $(cat "$tmp/err")"
  fi
  case "$2" in
    ran) [ -e "$tmp/native.log" ] || fail "$1: the custody suites did not run: $(cat "$tmp/err")" ;;
    skipped) [ ! -e "$tmp/native.log" ] || fail "$1: an unrelated change ran the custody suites" ;;
  esac
}
native_gate lego/operator/go.sum ran
native_gate .github/workflows/native-base-image.yml ran
native_gate .github/workflows/native-backport-acceptance.yml ran
native_gate .github/scripts/native-oci-mirror.py ran
native_gate .github/workflows/native-base-image.yml ran fail
native_gate docs/b.md skipped
native_gate lego/operator/go.mod.orig skipped
native_gate .github/workflows/deploy.yml skipped

echo "ship-gates.test: ok"
