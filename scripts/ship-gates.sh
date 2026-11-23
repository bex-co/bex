#!/usr/bin/env bash
# ship-gates.sh — the checks /ship runs after committing and before pushing
# (w5/m112). Exits non-zero only when THIS push's own code fails:
#
#   1. Main's CI status is reported, never enforced (user decision
#      2026-10-21: never stop or wait because main is red). When the newest
#      completed (not cancelled, skipped or neutral) run on main of
#      `deploy (bex via Argo)` or of a test suite (each runs on every main push
#      touching its inputs) failed or timed out, the gate prints a warning
#      naming it so the caller pauses to fix CI first, then ships either way.
#      Pass --fixes-red-main when this push is that fix: it only changes the
#      message. One read per workflow, no waiting.
#   2. The push changes lego/backend, lego/types or the seeded authz model and
#      the backend suite fails against its real Postgres + OpenFGA + OpenBao
#      (scripts/backend-test-deps.sh), which local `go test` otherwise skips.
#      This one blocks: a failure here is the push's own defect, fixed before
#      it ships.
#   3. The push changes dashboard/ (markdown excluded) and the dashboard's own
#      CI checks fail: `yarn lint` (typecheck, eslint, knip) and `yarn test`,
#      about two minutes locally. Blocks: four red mains in two days were
#      dashboard pushes nothing checked before CI (w6/m159).
#   4. A commit in the push adds a build artifact or evidence dump: a blob over
#      1 MiB or a binary/archive under .pm/ (scripts/repo-artifact-guard.sh,
#      w6/100 D10). Blocks; this is also what stops a stale checkout from
#      replaying pre-rewrite history.
#   5. The push changes a file the native qualification records by name
#      (native-common-qualify.py source-paths), a .github/scripts native module
#      or a native-* workflow, and the native suites fail (about a minute).
#      Blocks: one-line native edits passed every local check and turned
#      native-common red for hours (w6/107).
#   6. The push changes a test source, a suite workflow or the checker, and a
#      file a suite's tests read is missing from that suite's trigger paths
#      (scripts/ci_test_inputs.py). Blocks: a change to the unlisted file would
#      skip the suite on every push until the next batched deploy.
#   7. A commit in the push carries what looks like a credential
#      (scripts/gitleaks.sh over base..HEAD). Blocks: the repository is
#      public, and this is the last point where it is not yet published
#      (ADR019 §6).
#   8. A pushed commit's message carries a skip-CI marker. Blocks: GitHub then
#      runs no workflow for the whole push, the secret scan and the test
#      suites included (07a4238f1 did, quoting one in its body).
#   9. The push adds a flag to a bex-built container's args under
#      lego/operator/config that the pinned image does not define
#      (scripts/manifest_flag_guard.py). Blocks: Argo applies the manifest at
#      the push while the image moves only at the next pin, so the running
#      binary exits on the unknown flag (f114fdd9f crash-looped the manager for
#      about 35 minutes, w8/331).
#  10. The push adds a backend migration numbered at or below the highest one
#      on the base (scripts/migration_order_guard.py). Blocks: a database
#      already past that number, production included, skips it for good while
#      every fresh CI database applies it (0283 landed below a live 0284, and
#      the audit log answered 500 until a repair migration, w4/701).
#  11. A tracked shell script at HEAD passes a production credential in a
#      curl header or user argument (scripts/secret-argv-validate.sh), where
#      `ps` shows it to every local user. Blocks: pass it on stdin through
#      scripts/lib/curl-auth.sh instead (w3/m472).
#  12. The push changes .pm/ and adds a milestone number another workstream
#      already uses or leaves an open/blocked task with an id: differing from
#      its path (scripts/pm-board-validate.py --head HEAD). Blocks before push
#      so a rebased .pm filing cannot turn references.yml red on main.
#  13. The push adds a workspace id under .pm/ or docs/ that is neither one of
#      bex's own fixtures (scripts/lib/first_party_workspaces.txt) nor marked
#      `(disposable)` on its line (scripts/workspace_id_guard.py). A warning,
#      never a stop: the board is public, so another tenant's id is hashed or
#      left out (.pm/AGENTS.md), but most ids a push adds are disposable QA and
#      drill workspaces.
#  14. The push changes a path cli-test.yml runs on (read from the workflow's
#      own push paths, less those the backend suite of gate 2 already covers)
#      and a step of its job fails, run with CI=true as a hosted runner sets
#      it: scripts/bex-cli-validate.sh, `go test ./...` in lego/cli, and
#      `go test ./internal/cliauth ./internal/api` in lego/backend (left to
#      gate 2 when that runs). Blocks: ed70568c4 turned test (bex CLI) red
#      with a test that fails only under CI=true, and nothing here ran the
#      CLI (w21/117).
#  15. The push changes a file scripts/test_cli_analytics.py or
#      scripts/test_product_analytics.py reads (the two analytics SQL files,
#      the Grafana values, an edited migration) or either test, and they fail
#      against the postgres:17 digest gitops.yml's cli-analytics job pins, in
#      a container of its own that every way out of the gate removes. Blocks:
#      ed1452f75 turned gitops.yml red through scripts/cli-analytics.sql
#      (w21/117).
#
# Gates 2, 3, 5, 6, 14 and 15 and the gofmt check run in a detached worktree at HEAD,
# never in the working tree: a gate run beside an edit in progress tested the
# edit (a false failure), and an uncommitted file a test needed would pass a
# push whose commit lacks it. One worktree per checkout, reused so Go's build
# cache and the dashboard's node_modules stay warm (the first run checks out
# the tree and installs from scratch); a gate already running from the same
# checkout makes the next one use a throwaway worktree.
# BEX_SHIP_GATES_TREE overrides the worktree path.
#
#   bash scripts/ship-gates.sh [--fixes-red-main] <base-ref>   # e.g. origin/main
set -euo pipefail

fixes_red_main=0
base=""
for arg in "$@"; do
  case "$arg" in
    --fixes-red-main) fixes_red_main=1 ;;
    *) base=$arg ;;
  esac
done
[ -n "$base" ] || {
  echo "usage: $0 [--fixes-red-main] <base-ref>" >&2
  exit 2
}
root=$(cd "$(dirname "$0")/.." && pwd)
cd "$root"

# Repository security settings (w3/m365): a warning, never a stop. Push
# protection is readable only with an admin token, which shippers hold and the
# daily github-repo-security-drift workflow's GITHUB_TOKEN does not, so this is
# where it is checked in full. It runs in the background, one try per call, and
# is read at the end; an unreadable answer (no gh, no network) is quiet.
security_out=$(mktemp)
GH_RETRY_ATTEMPTS=1 bash scripts/github-repo-security-drift.sh >"$security_out" 2>/dev/null &
security_pid=$!

# No --branch: that filter turns GitHub's run listing into a search, which can
# answer with a page of only old runs (it once blocked a green main on a
# month-old failure). These workflows also run on pull requests and manual
# dispatches, so filter the branch and event here and take the newest verdict
# by createdAt; the verdict filter is ci-red-streak.sh's. A batched deploy run
# that deploys nothing concludes success, so the suites, which since then run
# in their own workflows, are read directly. The reads run in parallel.
red_main=0
ci_runs=$(mktemp -d)
ci_pids=""
workflows="deploy.yml $(python3 -B scripts/ci_test_inputs.py --suites)"
for workflow in $workflows; do
  (gh run list --workflow "$workflow" --limit 20 \
    --json conclusion,status,headSha,headBranch,event,url,createdAt >"$ci_runs/$workflow" 2>/dev/null \
    || rm -f "$ci_runs/$workflow") &
  ci_pids="$ci_pids $!"
done
# shellcheck disable=SC2086 # one pid per word
wait $ci_pids
for workflow in $workflows; do
  if [ ! -s "$ci_runs/$workflow" ]; then
    echo "ship-gates: warning: could not read main's CI status (gh unavailable?)" >&2
    break
  fi
  latest=$(jq -r 'map(select(.headBranch == "main" and .event != "pull_request" and .status == "completed"
        and .conclusion != "cancelled" and .conclusion != "skipped" and .conclusion != "neutral"))
    | max_by(.createdAt)
    | if . == null then "" else "\(.conclusion) \(.headSha[0:9]) \(.createdAt) \(.url)" end' "$ci_runs/$workflow")
  if [ -z "$latest" ]; then
    [ "$workflow" != deploy.yml ] || echo "ship-gates: warning: no completed deploy run on main to judge" >&2
    continue
  fi
  read -r conclusion sha created url <<<"$latest"
  case "$conclusion" in
    failure | timed_out | startup_failure)
      echo "ship-gates: warning: main is red — $workflow run for $sha (created $created) ended $conclusion: $url" >&2
      red_main=1
      ;;
  esac
done
rm -rf "$ci_runs"
if [ "$red_main" = 1 ]; then
  if [ "$fixes_red_main" = 1 ]; then
    echo "ship-gates: this push is marked as the fix; shipping" >&2
  else
    echo "ship-gates: fix CI first if you can (ship the fix alone, then resume); if it cannot be fixed now, ship anyway — never stop or wait" >&2
  fi
fi

bash scripts/repo-artifact-guard.sh "$base"
bash scripts/secret-argv-validate.sh HEAD

leak_rc=0
bash scripts/gitleaks.sh "$base..HEAD" || leak_rc=$?
if [ "$leak_rc" = 1 ]; then
  echo "ship-gates: a commit in this push carries what looks like a credential (findings above, redacted): remove it from the unpushed commits and rotate it if it is real; if its shape is never a credential, allowlist that shape in .gitleaks.toml with the reason" >&2
  exit 1
elif [ "$leak_rc" != 0 ]; then
  echo "ship-gates: the secret scan could not run; unscanned commits are not pushed" >&2
  exit 1
fi
# Captured, not piped: under pipefail an early-exiting grep can SIGPIPE git.
messages=$(git log --format=%B "$base"..HEAD)
if grep -qiE '\[(skip ci|ci skip|no ci|skip actions|actions skip)\]' <<<"$messages"; then
  echo "ship-gates: a commit message in this push carries a skip-CI marker, so GitHub would run no workflow for the push; reword it" >&2
  exit 1
fi
python3 -B scripts/manifest_flag_guard.py "$base" HEAD
python3 -B scripts/migration_order_guard.py "$base" HEAD
if [ -n "$(git diff --name-only "$base"..HEAD -- .pm)" ]; then
  python3 -B scripts/pm-board-validate.py --base "$base" --head HEAD >&2
fi
# Gate 13 warns only; the guard names each id itself.
python3 -B scripts/workspace_id_guard.py "$base" HEAD || true

# Which gates this push needs, from git objects only. Captured, not piped into
# grep: under pipefail an early-exiting grep can SIGPIPE git and make a large
# change look like no change. A bad base fails the assignment, and set -e
# fails the gate closed.
go_changed=$(git diff --name-only --diff-filter=ACMR "$base"..HEAD -- 'lego/*.go')
backend_changed=$(git diff --name-only "$base"..HEAD -- lego/backend lego/types deploy/gitops/authz ':(exclude)*.md')
dashboard_changed=$(git diff --name-only "$base"..HEAD -- dashboard ':(exclude)*.md')
all_changed=$(git diff --name-only "$base"..HEAD)
# The suite definitions and the native source list come from their modules (no
# copy to drift), captured first so a broken module fails the gate closed.
suite_inputs_changed=$(python3 -B scripts/ci_test_inputs.py --affected <<<"$all_changed")
native_sources=$(python3 -B .github/scripts/native-common-qualify.py source-paths)
native_changed=$({
  grep -xF -f <(printf '%s\n' "$native_sources") <<<"$all_changed"
  grep -E '^\.github/(scripts/[^/]*native[^/]*|workflows/native-[^/]*)$' <<<"$all_changed"
} || true)
# Gate 14: the paths cli-test.yml runs on, from the workflow itself so a path
# added there is never missed here, less what gate 2 covers: the backend suite
# runs lego/backend/internal and the authz model in full, and the CLI suite
# reads none of it.
cli_triggered=$(python3 -B scripts/ci_test_inputs.py --triggered .github/workflows/cli-test.yml <<<"$all_changed")
cli_changed=$(grep -vxF -f <(printf '%s\n' "$backend_changed") <<<"$cli_triggered" || true)
# Gate 15: what the cli-analytics job's two test files read. gitops.yml's own
# paths are all of lego/, scripts/ and deploy/ and would start Postgres on
# nearly every push. The tests apply a fixed list of migrations, so only an
# edit of an existing one is a read, never a new migration.
analytics_changed=$({
  git diff --name-only "$base"..HEAD -- scripts/cli-analytics.sql scripts/product-analytics.sql \
    scripts/test_cli_analytics.py scripts/test_product_analytics.py deploy/gitops/base/values/grafana.values.yaml
  git diff --name-only --diff-filter=MDR "$base"..HEAD -- lego/backend/internal/store/migrations
})

# The longest any one step that can wait on Docker or a test may run, so a
# hung pull, container or test cannot hang the gate; 15 minutes covers a cold
# pull of the Postgres image.
step_timeout=${BEX_SHIP_GATES_STEP_TIMEOUT:-900}
# timeout(1) is not on macOS: run a command for at most <seconds>, then kill it
# and fail 124 as timeout does.
bounded() {
  python3 -B -c 'import subprocess, sys
try:
    sys.exit(subprocess.run(sys.argv[2:], timeout=float(sys.argv[1])).returncode)
except subprocess.TimeoutExpired:
    shown = " ".join(sys.argv[2:4])
    print(f"ship-gates: {shown} ... still running after {sys.argv[1]}s; killed", file=sys.stderr)
    sys.exit(124)
except OSError as exc:
    print(f"ship-gates: cannot run {sys.argv[2]}: {exc}", file=sys.stderr)
    sys.exit(127)' "$@"
}

tree=""
lock=""
throwaway=""
analytics_db=""
release_tree() {
  # The analytics Postgres goes however the gate ends: a failed suite, a
  # signal or a timeout.
  if [ -n "$analytics_db" ]; then
    bounded 60 docker rm -f -v "$analytics_db" >/dev/null 2>&1 || true
  fi
  if [ -n "$throwaway" ]; then
    git -C "$root" worktree remove --force "$tree" >/dev/null 2>&1 || true
    rm -rf "$throwaway"
  fi
  [ -z "$lock" ] || rm -rf "$lock"
}
trap release_tree EXIT
common_dir() { (cd "$1" && cd "$(git rev-parse --git-common-dir)" && pwd -P) 2>/dev/null; }
# Every gate below that reads the tree must appear here.
if [ -n "$go_changed$backend_changed$dashboard_changed$suite_inputs_changed$native_changed$cli_changed$analytics_changed" ]; then
  # Keyed on the repository, not the checkout: linked worktrees (agent
  # checkouts) share one warm worktree instead of each leaving its own.
  common=$(common_dir .)
  tree=${BEX_SHIP_GATES_TREE:-"${XDG_CACHE_HOME:-$HOME/.cache}/bex-ship-gates/$(printf '%s' "$common" | shasum | cut -c1-8)"}
  mkdir -p "$(dirname "$tree")"
  # One gate at a time per worktree: a lock whose holder has exited is stale,
  # and a live one sends this gate to a throwaway worktree.
  if mkdir "$tree.lock" 2>/dev/null \
    || { ! kill -0 "$(cat "$tree.lock/pid" 2>/dev/null)" 2>/dev/null && rm -rf "$tree.lock" && mkdir "$tree.lock" 2>/dev/null; }; then
    lock="$tree.lock"
    echo $$ >"$lock/pid"
  else
    throwaway=$(mktemp -d)
    tree="$throwaway/tree"
  fi
  head=$(git rev-parse HEAD)
  if [ "$(common_dir "$tree")" = "$common" ]; then
    git -C "$tree" checkout -q --force --detach "$head"
    # Untracked files from an earlier commit go; ignored caches stay.
    git -C "$tree" clean -qfd
  else
    rm -rf "$tree"
    git worktree prune
    git worktree add -q --force --detach "$tree" "$head"
  fi
  echo "ship-gates: testing $(git rev-parse --short HEAD) in $tree" >&2
  cd "$tree"
fi

# The Go files this push changes must be gofmt-clean: only the operator's
# golangci-lint config runs the formatters, so backend, types and cli files
# drifted without any gate noticing (w21/060). A failing gofmt fails the gate
# closed.
# gofmt comes from the toolchain CI's go lint formats with, lego/cli/go.mod's go
# line (go-lint.yml, and the Makefile's GO_LINT_TOOLCHAIN for fmt-check), never
# from PATH: gofmt's output changes between Go releases, and an older PATH gofmt
# refused main's own formatting (w7/066).
if [ -n "$go_changed" ]; then
  gofmt_bin="$(GOTOOLCHAIN="go$(sed -n 's/^go //p' lego/cli/go.mod)" go env GOROOT)/bin/gofmt" || {
    echo "ship-gates: cannot resolve the pinned toolchain's gofmt (lego/cli/go.mod)" >&2
    exit 1
  }
  unformatted=$(xargs "$gofmt_bin" -l <<<"$go_changed")
  if [ -n "$unformatted" ]; then
    echo "ship-gates: gofmt would change these files; run $gofmt_bin -w on them and amend:" >&2
    printf '%s\n' "$unformatted" >&2
    exit 1
  fi
fi
if [ -n "$suite_inputs_changed" ]; then
  python3 -B scripts/ci_test_inputs.py >&2
fi
if [ -n "$analytics_changed" ]; then
  echo "ship-gates: CLI/product analytics inputs changed — running their SQL suites against the CI-pinned Postgres" >&2
  # The digest gitops.yml's cli-analytics job runs, read from it so the two
  # cannot drift.
  analytics_image=$(grep -m1 -oE 'postgres:17@sha256:[0-9a-f]{64}' .github/workflows/gitops.yml) || {
    echo "ship-gates: no postgres:17 digest pin in .github/workflows/gitops.yml" >&2
    exit 1
  }
  analytics_db="bex-ship-gates-analytics-$$"
  # Unpublished: the suites reach it through `docker exec`, as in CI.
  bounded "$step_timeout" docker run -d --rm --name "$analytics_db" --label bex-ship-gates=analytics \
    -e POSTGRES_PASSWORD=pw "$analytics_image" >/dev/null || {
    echo "ship-gates: could not start $analytics_image (is Docker running?)" >&2
    exit 1
  }
  analytics_ready=0
  for _ in $(seq 1 90); do
    if bounded 10 docker exec "$analytics_db" pg_isready -U postgres -h 127.0.0.1 >/dev/null 2>&1; then
      analytics_ready=1
      break
    fi
    sleep 1
  done
  [ "$analytics_ready" = 1 ] || {
    echo "ship-gates: the analytics Postgres did not become ready" >&2
    exit 1
  }
  BEX_CLI_ANALYTICS_TEST_CONTAINER="$analytics_db" bounded "$step_timeout" \
    python3 -B -m unittest scripts/test_cli_analytics.py scripts/test_product_analytics.py
  bounded 60 docker rm -f -v "$analytics_db" >/dev/null 2>&1 || true
  analytics_db=""
fi
if [ -n "$cli_changed" ]; then
  echo "ship-gates: bex CLI inputs changed — running cli-test.yml's steps with CI=true" >&2
  (
    # As a hosted runner runs them: CI=true, which ed70568c4's test failed
    # only under, and no cluster of the caller's.
    export CI=true
    unset KUBECONFIG
    bash scripts/bex-cli-validate.sh
    (cd lego/cli && go test ./...)
    # Gate 2 runs these two packages in full, against real dependencies.
    if [ -z "$backend_changed" ]; then
      (cd lego/backend && go test ./internal/cliauth ./internal/api)
    fi
  )
fi
if [ -n "$backend_changed" ]; then
  echo "ship-gates: backend/types/authz changed — running the backend suite against real dependencies" >&2
  bash scripts/backend-test-deps.sh up
  eval "$(bash scripts/backend-test-deps.sh env)"
  # No KUBECONFIG: with the test database set, it would point live-acceptance
  # tests at whatever cluster the caller has open.
  # The CRD contract's own tests, as backend-test.yml runs them (w4/589).
  (cd lego/types && GOWORK=off go test ./...)
  (cd lego/backend && env -u KUBECONFIG GOWORK=off BEX_TEST_REQUIRE_DEPS=1 go test -p 1 ./...)
  # Only on success: a failed suite leaves its database up to inspect.
  bash scripts/backend-test-deps.sh down
fi
if [ -n "$dashboard_changed" ]; then
  echo "ship-gates: dashboard changed — running its lint (typecheck, eslint, knip) and tests" >&2
  install_log=$(mktemp)
  (cd dashboard && yarn install --immutable >"$install_log" 2>&1) || {
    cat "$install_log" >&2
    exit 1
  }
  rm -f "$install_log"
  (cd dashboard && yarn lint && yarn test)
fi
if [ -n "$native_changed" ]; then
  echo "ship-gates: native qualification sources changed — running the native suites" >&2
  python3 -B -m unittest discover -s .github/scripts -p 'test_native_oci_*.py'
fi
security_rc=0
wait "$security_pid" || security_rc=$?
cd "$root"
if [ "$security_rc" -eq 1 ]; then
  echo "ship-gates: warning: the repository's secret scanning or main's history protection drifted (w3/m365):" >&2
  grep '^DRIFT' "$security_out" >&2 || true
fi
rm -f "$security_out"
echo "ship-gates: ok" >&2
