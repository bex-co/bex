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
cp "$root/scripts/ship-gates.sh" "$root/scripts/repo-artifact-guard.sh" "$root/scripts/ci_test_inputs.py" "$root/scripts/gitleaks.sh" "$root/scripts/manifest_flag_guard.py" "$root/scripts/migration_order_guard.py" "$root/scripts/pm-board-validate.py" "$root/scripts/pm_gates.py" "$root/scripts/deploy_latency.py" "$root/scripts/secret-argv-validate.sh" "$root/scripts/production-secrets.txt" "$root/scripts/workspace_id_guard.py" "$repo/scripts/"
cp "$root/.github/scripts/native-common-qualify.py" "$repo/.github/scripts/"
# Stands in for the native suites: logs that it ran, fails on demand.
cat >"$repo/.github/scripts/test_native_oci_stub.py" <<PY
import os, unittest
class Native(unittest.TestCase):
    def test_native(self):
        open("$tmp/native.log", "a").write("ran\\n")
        self.assertFalse(os.environ.get("NATIVE_FAIL"))
PY
git -C "$repo" init -q -b main
commit() { git -C "$repo" add -A && git -C "$repo" -c user.name=t -c user.email=t@t -c core.hooksPath=/dev/null -c commit.gpgsign=false commit -qm "$1"; }
echo '{}' >"$repo/dashboard/package.json"
# What the CLI and analytics gates read from the tree they test: cli-test.yml's
# own trigger paths, gitops.yml's Postgres pin, the CLI pin check and the two
# analytics suites, as stubs that log what ran and fail on demand, and the
# backend dependencies' script.
cp "$root/.github/workflows/cli-test.yml" "$repo/.github/workflows/"
printf 'steps:\n  - run: |\n      image=postgres:17@sha256:%s\n' "$(printf '0123456789abcdef%.0s' 1 2 3 4)" >"$repo/.github/workflows/gitops.yml"
mkdir -p "$repo/lego/types" "$repo/lego/backend"
echo stub >"$repo/lego/types/doc.txt"
echo stub >"$repo/lego/backend/doc.txt"
cat >"$repo/scripts/bex-cli-validate.sh" <<STUB
#!/bin/sh
echo "validate CI=\${CI:-} KUBECONFIG=\${KUBECONFIG:-}" >>"$tmp/cli.log"
[ ! -e "$tmp/validate-fails" ]
STUB
cat >"$repo/scripts/backend-test-deps.sh" <<STUB
#!/bin/sh
echo "deps \$1" >>"$tmp/cli.log"
[ "\$1" != env ] || echo "export BEX_TEST_DB_URI=stub"
STUB
for suite in cli product; do
  cat >"$repo/scripts/test_${suite}_analytics.py" <<PY
import os, time, unittest
class Analytics(unittest.TestCase):
    def test_analytics(self):
        open("$tmp/analytics.log", "a").write("$suite %s\\n" % os.environ.get("BEX_CLI_ANALYTICS_TEST_CONTAINER"))
        if os.path.exists("$tmp/analytics-hangs"):
            time.sleep(60)
        self.assertFalse(os.path.exists("$tmp/analytics-fails"))
PY
done
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
# Stands in for the pinned Gitleaks binary: logs its arguments; a finding or
# a scanner error on demand.
cat >"$tmp/gitleaks" <<STUB
#!/bin/sh
echo "\$*" >>"$tmp/gitleaks.log"
[ ! -e "$tmp/leak" ] || exit 3
[ ! -e "$tmp/scanerr" ] || exit 1
exit 0
STUB
chmod +x "$tmp/gitleaks"
gate() { (cd "$repo" && BEX_SHIP_GATES_TREE="$tmp/tree" BEX_GITLEAKS_BIN="$tmp/gitleaks" PATH="$tmp:$PATH" bash scripts/ship-gates.sh "$@") 2>"$tmp/err"; }
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

# The secret scan (gate 7): the pushed commits are scanned, a finding blocks,
# and a scanner that cannot run fails closed.
grep -q -- "--log-opts=HEAD~1..HEAD" "$tmp/gitleaks.log" || fail "the gate did not scan the pushed commits: $(cat "$tmp/gitleaks.log")"
touch "$tmp/leak"
if gate HEAD~1; then fail "a pushed credential did not block the push"; fi
grep -q "looks like a credential" "$tmp/err" || fail "a finding was not named: $(cat "$tmp/err")"
rm "$tmp/leak"
touch "$tmp/scanerr"
if gate HEAD~1; then fail "a scan that could not run let the push through"; fi
grep -q "secret scan could not run" "$tmp/err" || fail "a scanner error was not named: $(cat "$tmp/err")"
rm "$tmp/scanerr"

# A skip-CI marker anywhere in a pushed commit's message (gate 8) blocks: the
# push would start no workflow at all.
echo more >>"$repo/docs/a.md"
commit "docs: a body that quotes [skip ci] by mistake"
if gate HEAD~1; then fail "a skip-CI marker in a pushed commit message did not block the push"; fi
grep -q "skip-CI marker" "$tmp/err" || fail "the skip-CI marker was not named: $(cat "$tmp/err")"
echo more >>"$repo/docs/a.md"
commit "docs: no marker"
gate HEAD~1 || fail "a message without a marker failed the gate: $(cat "$tmp/err")"

# The native gate (w6/107): a file the qualification source inventory names, a
# native workflow or a native module runs the native suites, a failure blocks,
# and anything else runs nothing.
native_gate() { # <path> <expect: ran|skipped> [NATIVE_FAIL]
  mkdir -p "$(dirname "$repo/$1")"
  echo "$RANDOM" >>"$repo/$1"
  commit "$1"
  rm -f "$tmp/native.log"
  if NATIVE_FAIL="${3:-}" gate HEAD~1; then
    [ -z "${3:-}" ] || fail "$1: a failing native suite did not block the push"
  else
    [ -n "${3:-}" ] || fail "$1: a passing push failed the gate: $(cat "$tmp/err")"
  fi
  case "$2" in
    ran) [ -e "$tmp/native.log" ] || fail "$1: the native suites did not run: $(cat "$tmp/err")" ;;
    skipped) [ ! -e "$tmp/native.log" ] || fail "$1: an unrelated change ran the native suites" ;;
  esac
}
native_gate lego/operator/go.sum ran
native_gate deploy/native-common/Dockerfile ran
native_gate lego/operator/internal/build/toolchain-freshness.json ran
native_gate .github/workflows/native-base-image.yml ran
native_gate .github/workflows/native-backport-acceptance.yml ran
native_gate .github/scripts/native-oci-mirror.py ran
native_gate .github/workflows/native-base-image.yml ran fail
native_gate docs/b.md skipped
native_gate lego/operator/go.mod.orig skipped
native_gate .github/workflows/deploy.yml skipped

# The gofmt check (w7/066) formats with the toolchain CI's go lint uses,
# lego/cli/go.mod's, never the gofmt first on PATH: a PATH gofmt that rejects
# every file stands in for an older Go, and a stubbed go answers GOROOT with a
# gofmt that rejects only files marked UNFORMATTED.
mkdir -p "$tmp/gostub" "$tmp/goroot/bin" "$repo/lego/cli"
printf '#!/bin/sh\nprintf "%%s\\n" "$@"\n' >"$tmp/gofmt"
cat >"$tmp/gostub/go" <<STUB
#!/bin/sh
echo "\$GOTOOLCHAIN \$*" >>"$tmp/go.log"
[ ! -e "$tmp/noroot" ] || exit 1
[ "\$*" != "env GOROOT" ] || echo "$tmp/goroot"
STUB
cat >"$tmp/goroot/bin/gofmt" <<'STUB'
#!/bin/sh
shift
grep -l UNFORMATTED "$@" || true
STUB
chmod +x "$tmp/gofmt" "$tmp/gostub/go" "$tmp/goroot/bin/gofmt"
fmt_gate() { PATH="$tmp/gostub:$PATH" gate "$@"; }
printf 'module example.com/cli\n\ngo 1.27.3\n' >"$repo/lego/cli/go.mod"
echo 'package x' >"$repo/lego/x.go"
commit "a formatted go file"
rm -f "$tmp/go.log"
fmt_gate HEAD~1 || fail "a file the pinned gofmt accepts was refused: $(cat "$tmp/err")"
grep -qx "go1.27.3 env GOROOT" "$tmp/go.log" || fail "gofmt was not taken from lego/cli/go.mod's toolchain: $(cat "$tmp/go.log" 2>/dev/null)"
touch "$tmp/noroot"
if fmt_gate HEAD~1; then fail "an unresolvable toolchain let a go change through"; fi
grep -q "cannot resolve the pinned toolchain's gofmt" "$tmp/err" || fail "an unresolvable toolchain was not named: $(cat "$tmp/err")"
rm "$tmp/noroot"
echo '// UNFORMATTED' >>"$repo/lego/x.go"
commit "an unformatted go file"
if fmt_gate HEAD~1; then fail "a file the pinned gofmt rejects passed the gate"; fi
grep -q "run $tmp/goroot/bin/gofmt -w on them" "$tmp/err" || fail "the refusal did not name the pinned gofmt: $(cat "$tmp/err")"
grep -qx "lego/x.go" "$tmp/err" || fail "the refusal did not name the file: $(cat "$tmp/err")"

# The CLI gate (gate 14, w21/117): a push that changes a path cli-test.yml runs
# on runs that job's steps as a hosted runner does, with CI=true and no
# cluster of the caller's, and a failing step blocks. The path list is the
# workflow's own. What the backend suite already runs (lego/backend/internal,
# the authz model) never starts it, and when both run, the backend suite alone
# runs the cliauth and api packages. The stubs log their working directory.
# A runner's own CI=true must not reach the stubs' record of what the gate set.
unset CI
mkdir -p "$tmp/clistub"
cat >"$tmp/clistub/go" <<STUB
#!/bin/sh
echo "\${PWD#$tmp/tree/} CI=\${CI:-} KUBECONFIG=\${KUBECONFIG:-} go \$*" >>"$tmp/cli.log"
[ "\$*" != "\${CLI_FAIL:-}" ] || exit 1
exit 0
STUB
cat >"$tmp/clistub/docker" <<STUB
#!/bin/sh
echo "\$*" >>"$tmp/docker.log"
case "\$1" in
  run) [ ! -e "$tmp/docker-run-fails" ] || exit 1 ;;
  exec) [ ! -e "$tmp/docker-not-ready" ] || exit 1 ;;
esac
exit 0
STUB
mkdir -p "$tmp/nosleep"
printf '#!/bin/sh\nexit 0\n' >"$tmp/nosleep/sleep"
chmod +x "$tmp/clistub/go" "$tmp/clistub/docker" "$tmp/nosleep/sleep"
cli_gate() { KUBECONFIG=/live/cluster PATH="$tmp/clistub:$PATH" gate "$@"; }
cli_push() { # <path> <content line>
  mkdir -p "$(dirname "$repo/$1")"
  echo "$2" >>"$repo/$1"
  commit "$1"
  rm -f "$tmp/cli.log" "$tmp/docker.log" "$tmp/analytics.log"
}
cli_push docs/cli-compatibility-checklist.md "a checklist line"
cli_gate HEAD~1 || fail "a passing CLI push failed the gate: $(cat "$tmp/err")"
for want in "validate CI=true KUBECONFIG=" "lego/cli CI=true KUBECONFIG= go test ./..." \
  "lego/backend CI=true KUBECONFIG= go test ./internal/cliauth ./internal/api"; do
  grep -qxF "$want" "$tmp/cli.log" || fail "the CLI gate did not run '$want': $(cat "$tmp/cli.log")"
done
[ ! -e "$tmp/docker.log" ] || fail "a CLI push started Docker: $(cat "$tmp/docker.log")"
rm -f "$tmp/cli.log"
CLI_FAIL="test ./..." cli_gate HEAD~1 && fail "a failing CLI suite did not block the push"
grep -q "ship-gates: ok" "$tmp/err" && fail "a failing CLI suite still reported ok"
grep -q "internal/cliauth" "$tmp/cli.log" && fail "the adapter packages ran after the CLI suite failed"
rm -f "$tmp/cli.log"
touch "$tmp/validate-fails"
cli_gate HEAD~1 && fail "a failing upstream pin check did not block the push"
grep -q "go test" "$tmp/cli.log" && fail "the CLI suite ran after the pin check failed"
rm -f "$tmp/validate-fails" "$tmp/cli.log"
CLI_FAIL="test ./internal/cliauth ./internal/api" cli_gate HEAD~1 && fail "failing adapter tests did not block the push"
# A path the workflow does not list starts nothing; one it lists that the
# backend suite does not run (a markdown note under internal/) does.
cli_push docs/c.md "unrelated"
cli_gate HEAD~1 || fail "an unrelated push failed the gate: $(cat "$tmp/err")"
[ ! -e "$tmp/cli.log" ] || fail "an unrelated push ran the CLI gate: $(cat "$tmp/cli.log")"
cli_push lego/backend/internal/api/NOTES.md "a note the backend suite excludes"
cli_gate HEAD~1 || fail "a backend note push failed the gate: $(cat "$tmp/err")"
grep -qxF "lego/cli CI=true KUBECONFIG= go test ./..." "$tmp/cli.log" || fail "a path cli-test.yml lists did not start the CLI gate: $(cat "$tmp/cli.log")"
grep -q "deps " "$tmp/cli.log" && fail "a markdown note started the backend suite"
# The backend suite covers lego/backend/internal alone: no CLI, and with a CLI
# change beside it no second run of the adapter packages.
cli_push lego/backend/internal/api/x.txt "a backend input"
cli_gate HEAD~1 || fail "a backend push failed the gate: $(cat "$tmp/err")"
grep -qxF "lego/backend CI= KUBECONFIG= go test -p 1 ./..." "$tmp/cli.log" || fail "the backend suite did not run: $(cat "$tmp/cli.log")"
grep -q "lego/cli " "$tmp/cli.log" && fail "a backend-only push ran the CLI suite: $(cat "$tmp/cli.log")"
echo "a CLI doc" >>"$repo/lego/cli/README.md"
echo "more backend" >>"$repo/lego/backend/internal/api/x.txt"
commit "a CLI doc beside a backend input"
rm -f "$tmp/cli.log"
cli_gate HEAD~1 || fail "a CLI plus backend push failed the gate: $(cat "$tmp/err")"
grep -qxF "lego/cli CI=true KUBECONFIG= go test ./..." "$tmp/cli.log" || fail "the CLI suite did not run beside the backend suite: $(cat "$tmp/cli.log")"
grep -q "internal/cliauth" "$tmp/cli.log" && fail "the adapter packages ran twice: $(cat "$tmp/cli.log")"
grep -q "go test -p 1 ./..." "$tmp/cli.log" || fail "the backend suite did not run beside the CLI suite: $(cat "$tmp/cli.log")"

# The analytics gate (gate 15, w21/117): a push that changes a file the two
# analytics suites read runs them against a Postgres container of its own, from
# the digest gitops.yml pins, and removes it however the gate ends. A new
# migration is no read (the suites apply a fixed list); an edited one is.
pin="postgres:17@sha256:$(printf '0123456789abcdef%.0s' 1 2 3 4)"
analytics_container() { sed -n 's/^run -d --rm --name \([^ ]*\) .*/\1/p' "$tmp/docker.log"; }
cli_push scripts/cli-analytics.sql "SELECT 1;"
cli_gate HEAD~1 || fail "a passing analytics push failed the gate: $(cat "$tmp/err")"
name=$(analytics_container)
case "$name" in bex-ship-gates-analytics-*) ;; *) fail "no analytics container was started: $(cat "$tmp/docker.log")" ;; esac
grep -qxF "run -d --rm --name $name --label bex-ship-gates=analytics -e POSTGRES_PASSWORD=pw $pin" "$tmp/docker.log" || fail "the container is not the pinned image: $(cat "$tmp/docker.log")"
grep -qx "exec $name pg_isready -U postgres -h 127.0.0.1" "$tmp/docker.log" || fail "the gate did not wait for Postgres: $(cat "$tmp/docker.log")"
grep -qx "rm -f -v $name" "$tmp/docker.log" || fail "the container was left running: $(cat "$tmp/docker.log")"
grep -qx "cli $name" "$tmp/analytics.log" && grep -qx "product $name" "$tmp/analytics.log" || fail "both suites did not run against the container: $(cat "$tmp/analytics.log")"
grep -q "go test" "$tmp/cli.log" 2>/dev/null && fail "an analytics push ran the CLI suite"
touch "$tmp/analytics-fails"
rm -f "$tmp/docker.log"
cli_gate HEAD~1 && fail "failing analytics tests did not block the push"
grep -q "ship-gates: ok" "$tmp/err" && fail "failing analytics tests still reported ok"
grep -qx "rm -f -v $(analytics_container)" "$tmp/docker.log" || fail "a failed analytics run left its container: $(cat "$tmp/docker.log")"
rm -f "$tmp/analytics-fails" "$tmp/docker.log" "$tmp/analytics.log"
touch "$tmp/docker-run-fails"
cli_gate HEAD~1 && fail "a Postgres that could not start did not block the push"
grep -q "could not start $pin" "$tmp/err" || fail "the failed start was not named: $(cat "$tmp/err")"
[ ! -e "$tmp/analytics.log" ] || fail "the suites ran without a database"
rm -f "$tmp/docker-run-fails"
touch "$tmp/docker-not-ready"
(PATH="$tmp/nosleep:$PATH" cli_gate HEAD~1) && fail "a Postgres that never became ready did not block the push"
grep -q "the analytics Postgres did not become ready" "$tmp/err" || fail "readiness was not named: $(cat "$tmp/err")"
rm -f "$tmp/docker-not-ready" "$tmp/docker.log" "$tmp/analytics.log"
# A hung suite is killed at the step limit, and the gate still removes the container.
touch "$tmp/analytics-hangs"
(BEX_SHIP_GATES_STEP_TIMEOUT=2 cli_gate HEAD~1) && fail "a hung analytics suite did not block the push"
rm -f "$tmp/analytics-hangs"
grep -q "still running after 2s; killed" "$tmp/err" || fail "the timeout was not named: $(cat "$tmp/err")"
grep -qx "rm -f -v $(analytics_container)" "$tmp/docker.log" || fail "a hung analytics run left its container: $(cat "$tmp/docker.log")"
# Which files start it.
for changed in scripts/product-analytics.sql scripts/test_cli_analytics.py scripts/test_product_analytics.py deploy/gitops/base/values/grafana.values.yaml; do
  cli_push "$changed" "# $RANDOM"
  cli_gate HEAD~1 || fail "$changed: a passing push failed the gate: $(cat "$tmp/err")"
  [ -s "$tmp/analytics.log" ] || fail "$changed: the analytics suites did not run"
done
cli_push scripts/cli-analytics-bootstrap.sh "# not a read"
cli_gate HEAD~1 || fail "an unrelated script failed the gate: $(cat "$tmp/err")"
[ ! -e "$tmp/docker.log" ] || fail "an unrelated script started Docker: $(cat "$tmp/docker.log")"
migrations="$repo/lego/backend/internal/store/migrations"
mkdir -p "$migrations"
echo 'SELECT 1;' >"$migrations/0001_core.up.sql"
commit "a migration the suites apply"
rm -f "$tmp/cli.log" "$tmp/docker.log" "$tmp/analytics.log"
echo 'SELECT 2;' >"$migrations/0002_later.up.sql"
commit "a new migration"
cli_gate HEAD~1 || fail "a new migration failed the gate: $(cat "$tmp/err")"
[ ! -e "$tmp/docker.log" ] || fail "a new migration, which the suites never apply, started Docker"
echo 'SELECT 3;' >>"$migrations/0001_core.up.sql"
commit "an edited migration"
cli_gate HEAD~1 || fail "an edited migration failed the gate: $(cat "$tmp/err")"
[ -s "$tmp/analytics.log" ] || fail "an edited migration did not run the analytics suites"
rm -f "$tmp/cli.log" "$tmp/docker.log" "$tmp/analytics.log"
git -C "$repo" rm -qr lego/backend/internal/store lego/backend/internal/api
commit "the migration and backend fixtures go"
git -C "$repo" rm -q scripts/cli-analytics-bootstrap.sh
commit "the bootstrap fixture goes"

# The migration order gate (gate 10, w4/701): a migration numbered below the
# base's highest blocks before any suite runs, naming the file and the rule.
# The passing side is scripts/test_migration_order_guard.py's (a passing
# backend change would start the real-DB suite here).
migrations="$repo/lego/backend/internal/store/migrations"
mkdir -p "$migrations"
echo 'SELECT 1;' >"$migrations/0284_cron_minimum.up.sql"
commit "a live migration"
echo 'SELECT 1;' >"$migrations/0283_audit_source_ip_user_agent.up.sql"
commit "a migration rebased below it"
if gate HEAD~1; then fail "a migration numbered below the base's highest passed the gate"; fi
grep -q "0283_audit_source_ip_user_agent.up.sql numbers at or below 0284_cron_minimum.up.sql" "$tmp/err" || fail "the low migration was not named: $(cat "$tmp/err")"
grep -q "renumber above 0284" "$tmp/err" || fail "the refusal did not say how to fix it: $(cat "$tmp/err")"
grep -q "testing " "$tmp/err" && fail "the backend suite was reached before the migration order refused the push"

# The workspace id warning (gate 13): a push that adds an id that is neither
# listed nor marked disposable ships with a warning naming it; a marked one
# ships quietly.
mkdir -p "$repo/.pm/w9"
printf '# 001\n\nRead in workspace `tea-d0tenant000000000002`.\n' >"$repo/.pm/w9/001.md"
commit "w9 names an unlisted workspace"
gate HEAD~1 || fail "an unlisted workspace id stopped the ship: $(cat "$tmp/err")"
grep -q "warning: .pm/w9/001.md:3 adds workspace tea-d0tenant000000000002" "$tmp/err" || fail "the unlisted id was not named: $(cat "$tmp/err")"
grep -q "ship-gates: ok" "$tmp/err" || fail "the warning did not let the ship finish: $(cat "$tmp/err")"
printf -- '- Drill workspace `tea-d0tenant000000000003` (disposable).\n' >>"$repo/.pm/w9/001.md"
commit "w9 names a disposable workspace"
gate HEAD~1 || fail "a disposable workspace id stopped the ship: $(cat "$tmp/err")"
if grep -q "workspace_id_guard" "$tmp/err"; then fail "a marked disposable id warned: $(cat "$tmp/err")"; fi

# The .pm board gate (gate 12): a commit that adds a milestone number another
# workstream already uses or leaves a task with a mismatched id blocks before
# push, while an uncommitted board edit in the working tree does not affect
# a clean commit.
mkdir -p "$repo/.pm/w3/m505"
echo '# w3 m505' >"$repo/.pm/w3/m505/README.md"
printf -- '---\nid: w3/m505/t001\ntitle: t\nstatus: todo\n---\n' >"$repo/.pm/w3/m505/t001.md"
commit "w3 files m505"
mkdir -p "$repo/.pm/w4/m510"
echo '# w4 m510' >"$repo/.pm/w4/m510/README.md"
printf -- '---\nid: w4/m510/t001\ntitle: t\nstatus: todo\n---\n' >"$repo/.pm/w4/m510/t001.md"
commit "w4 files free m510"
# Uncommitted collision in the working tree must not break a clean HEAD.
mkdir -p "$repo/.pm/w4/m505"
echo '# uncommitted dup' >"$repo/.pm/w4/m505/README.md"
gate HEAD~1 || fail "a clean .pm commit failed because of an uncommitted collision: $(cat "$tmp/err")"
commit "w4 commits colliding m505"
if gate HEAD~1; then fail "a colliding milestone number passed the gate"; fi
grep -q ".pm/w4/m505 adds milestone m505, already used by w3" "$tmp/err" || fail "the colliding milestone was not named: $(cat "$tmp/err")"
git -C "$repo" rm -qr .pm/w4/m505
printf -- '---\nid: w4/m999/t001\ntitle: t\nstatus: todo\n---\n' >"$repo/.pm/w4/m510/t001.md"
commit "w4 breaks task id"
if gate HEAD~1; then fail "a mismatched task id passed the gate"; fi
grep -q ".pm/w4/m510/t001.md has id: w4/m999/t001, want w4/m510/t001" "$tmp/err" || fail "the mismatched task id was not named: $(cat "$tmp/err")"

echo "ship-gates.test: ok"
