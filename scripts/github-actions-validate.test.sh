#!/usr/bin/env bash
# Self-test for scripts/github-actions-validate.sh (w7/m65 t002/t006). The
# anti-tautology pattern (m60/m62): a guard with no proven red case proves
# nothing. This exercises the portable SHA-pin/runtime checks against throwaway
# fixture trees (via WORKFLOWS_DIR) — a mutable tag, a too-short/too-long SHA,
# and a Node 20 runtime must turn it red; a 40-hex pin, a local ./ reusable ref,
# and a fully-commented tag ref must stay green — then confirms the real tree
# passes end-to-end and the dashboard audit gate + dependabot config are present.
set -euo pipefail

here="$(cd "$(dirname "$0")" && pwd)"
SCRIPT="$here/github-actions-validate.sh"
root="$(cd "$here/.." && pwd)"
[ -x "$SCRIPT" ] || { echo "FAIL: $SCRIPT not executable" >&2; exit 1; }

fails=0
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

PINNED='azure/setup-kubectl@829323503d1be3d00ca8346e5391ca0b07a9ab0d # v5.1.0'
SHA40='3d3c42e5aac5ba805825da76410c181273ba90b1' # the 40-hex commit for v7.0.1

# assert <label> <want-rc> <workflow-body> [expected-stderr-substring]
# Writes the body to a fresh fixture dir, runs the validator over it via
# WORKFLOWS_DIR (absolute — the script cd's to repo root), checks the exit code,
# and (when given) that stderr names the offending ref.
assert() {
  local label="$1" want="$2" body="$3" needle="${4:-}"
  local dir="$tmp/wf-$RANDOM-$RANDOM"
  mkdir -p "$dir"
  printf '%s\n' "$body" >"$dir/ci.yml"
  set +e
  local err; err="$(WORKFLOWS_DIR="$dir" "$SCRIPT" 2>&1 >/dev/null)"
  local got=$?
  set -e
  if [ "$got" -ne "$want" ]; then
    echo "FAIL: $label — exit $got, want $want" >&2; fails=$((fails + 1)); return
  fi
  if [ -n "$needle" ] && ! printf '%s' "$err" | grep -qF "$needle"; then
    echo "FAIL: $label — stderr did not name '$needle'" >&2; fails=$((fails + 1)); return
  fi
  echo "ok: $label (exit $got)"
}

body() { printf 'jobs:\n  x:\n    steps:\n%s' "$1"; }

# GREEN: a 40-hex-pinned ref passes.
assert "pinned ref passes" 0 "$(body "      - uses: $PINNED")"
# GREEN: local ./ reusable-workflow ref is exempt (path ref to this repo).
assert "local ./ ref exempt" 0 "$(body "      - uses: ./.github/workflows/backend-test.yml
      - uses: $PINNED")"
# GREEN: a fully-commented tag ref is skipped (it never executes).
assert "commented tag ref skipped" 0 "$(body "      # - uses: actions/checkout@v7
      - uses: $PINNED")"

# RED: a mutable tag ref fails and is named in the message.
assert "tag ref fails" 1 "$(body "      - uses: actions/checkout@v7")" "actions/checkout@v7"
# RED: a 39-hex (too short) SHA-like ref fails.
assert "39-hex ref fails" 1 "$(body "      - uses: actions/checkout@${SHA40%?}")" "actions/checkout@"
# RED: a 41-hex (too long) SHA-like ref fails.
assert "41-hex ref fails" 1 "$(body "      - uses: actions/checkout@${SHA40}a")" "actions/checkout@"
# RED: an end-of-life Node 20 runtime fails (portable runtime check).
assert "node 20 fails" 1 "$(body "      - uses: $PINNED
        with:
          node-version: '20'")"
# RED: a direct `gh issue create`/`gh issue edit` fails, named by file:line;
# GREEN: the helper call and a commented mention pass (w6/m161).
assert "direct gh issue create fails" 1 "$(body "      - uses: $PINNED
      - run: gh issue create --title t --body b")" "ci.yml:5"
assert "direct gh issue edit fails" 1 "$(body "      - run: gh issue edit 1 --body b")" "ci.yml:4"
assert "tracking-issue helper passes" 0 "$(body "      # gh issue create is not called directly
      - uses: $PINNED
      - run: bash scripts/lib/tracking-issue.sh open --label l --title t --body-file f")"

# --- ADR083 amendment 2026-11-09 / #CI-RUNNERS: GitHub-hosted runners --------
ARM=ubuntu-24.04-arm
job_body() {
  printf 'jobs:\n  x:\n    runs-on: %s\n    steps:\n%s' "$1" "$2"
}
# GREEN: the two approved hosted labels, scalar form.
assert "ubuntu-24.04-arm passes" 0 "$(job_body "$ARM" "      - uses: $PINNED")"
assert "ubuntu-latest passes" 0 "$(job_body ubuntu-latest "      - uses: $PINNED")"
# RED: the retired self-hosted pools, in any form, and any mention at all.
assert "self-hosted pool label fails" 1 "$(job_body "[self-hosted, Linux, ARM64, bex-ci]" "      - uses: $PINNED")" \
  "must not mention self-hosted runners"
assert "self-hosted mention in a comment fails" 1 "$(job_body "$ARM" "      # runs on the self-hosted pool
      - uses: $PINNED")" "ci.yml:5"
# RED: any other label, a list, an expression, or a runner group.
assert "unapproved hosted label fails" 1 "$(job_body ubuntu-22.04 "      - uses: $PINNED")" \
  "runs-on must be the scalar ubuntu-24.04-arm (default) or ubuntu-latest"
assert "list-form runs-on fails" 1 "$(job_body "[$ARM]" "      - uses: $PINNED")" \
  "runs-on must be the scalar"
assert "expression runs-on fails" 1 "$(job_body "\${{ matrix.os }}" "      - uses: $PINNED")" \
  "runs-on must be the scalar"
assert "runner-group mapping fails" 1 "$(printf 'jobs:\n  x:\n    runs-on:\n      group: bex-ci\n      labels: [%s]\n    steps:\n      - uses: %s' "$ARM" "$PINNED")" \
  "runs-on must be the scalar"
# GREEN: hosted runners have sudo; the disk-cleanup step needs it.
assert "sudo step passes on hosted runners" 0 "$(job_body ubuntu-latest "      - run: sudo rm -rf /usr/share/dotnet
      - uses: $PINNED")"

# --- check 6a: credentials never reach a pull_request job ---------------------
pr_wf() { # <job if-line or empty> <job extra lines> [trigger]
  printf 'on:\n  %s:\njobs:\n  x:\n%b    runs-on: %s\n%b    steps:\n      - uses: %s' \
    "${3:-pull_request}" "${1:+    $1\n}" "$ARM" "$2" "$PINNED"
}
SAME_REPO="if: github.event_name != 'pull_request' || github.event.pull_request.head.repo.full_name == github.repository"
assert "secret in a PR job fails" 1 "$(pr_wf "$SAME_REPO" '    env:\n      T: ${{ secrets.PRODUCTION_TOKEN }}\n')" \
  "x: credential-bearing job must reject pull_request events"
assert "write token in a PR job fails" 1 "$(pr_wf "$SAME_REPO" '    permissions:\n      contents: write\n')" \
  "credential-bearing job must reject pull_request events"
assert "workflow write-all in a PR workflow fails" 1 "$(printf 'permissions: write-all\n%s' "$(pr_wf "$SAME_REPO" '')")" \
  "credential-bearing job must reject pull_request events"
assert "environment in a PR job fails" 1 "$(pr_wf "$SAME_REPO" '    environment: production-deploy\n')" \
  "credential-bearing job must reject pull_request events"
assert "toJSON(secrets) in a PR job fails" 1 "$(pr_wf "$SAME_REPO" '    env:\n      ALL: ${{ toJSON(secrets) }}\n')" \
  "credential-bearing job must reject pull_request events"
# RED: a same-repository gate alone still lets a branch PR read the secret.
assert "same-repository gate does not admit a credentialed PR job" 1 "$(pr_wf "if: github.event.pull_request.head.repo.full_name == github.repository" '    environment: production-deploy\n')" \
  "credential-bearing job must reject pull_request events"
# RED: pull_request_target needs its own exclusion.
assert "credentialed pull_request_target job fails" 1 "$(pr_wf "if: github.event_name != 'pull_request' || github.event.pull_request.head.repo.full_name == github.repository" '    environment: production-deploy\n' pull_request_target)" \
  "credential-bearing job must reject pull_request events"
# GREEN: the infra.yml terraform shape -- PR events excluded before scheduling.
assert "PR-excluded credentialed job passes" 0 "$(pr_wf "if: github.event_name != 'pull_request'" '    environment: production-infra\n    env:\n      T: ${{ secrets.PRODUCTION_TOKEN }}\n')"
# GREEN: an uncredentialed same-repository PR job, and credentials off PR workflows.
assert "uncredentialed PR job passes" 0 "$(pr_wf "$SAME_REPO" '')"
assert "secret in a push-only workflow passes" 0 "$(job_body "$ARM" "      - env:
          TOKEN: \${{ secrets.PRODUCTION_TOKEN }}
        run: ./deploy")"

# --- ADR080 F2 / w2/m89: secretless image build ------------------------------
BUILD_PUSH='docker/build-push-action@53b7df96c91f9c12dcc8a07bcb9ccacbed38856a # v7.3.0'
# RED: a deploy-class secret in an image-build job is exfiltratable by any
# poisoned build input — exactly the pre-split deploy.yml shape.
assert "deploy secret in image-build job fails" 1 "$(job_body "$ARM" "      - uses: $BUILD_PUSH
      - env:
          HCLOUD_TOKEN: \${{ secrets.HCLOUD_TOKEN }}
        run: ./deploy")" "image-build job must reference no secret beyond the registry-push GITHUB_TOKEN"
# RED: an environment gate on the build job would scope every environment
# secret into repo-controlled build code.
assert "environment gate on image-build job fails" 1 "$(printf 'jobs:\n  x:\n    runs-on: %s\n    environment: production-deploy\n    steps:\n      - uses: %s' "$ARM" "$BUILD_PUSH")" \
  "image-build job must not carry an environment gate"
# RED: a workflow-header secret leaks into every job, including the build.
assert "workflow-header secret with image build fails" 1 "$(printf 'env:\n  TOKEN: ${{ secrets.PRODUCTION_TOKEN }}\njobs:\n  x:\n    runs-on: %s\n    steps:\n      - uses: %s' "$ARM" "$BUILD_PUSH")" \
  "image-build job must reference no secret beyond the registry-push GITHUB_TOKEN"
# GREEN: the registry-push GITHUB_TOKEN is the build job's entire credential
# budget (the post-split deploy.yml build job shape).
assert "registry-only image-build job passes" 0 "$(job_body ubuntu-latest "      - env:
          REGISTRY_TOKEN: \${{ secrets.GITHUB_TOKEN }}
        run: docker login
      - uses: $BUILD_PUSH")"
# RED: the bracket secrets form GitHub also resolves must not slip past.
assert "bracket-form secret in image-build job fails" 1 "$(job_body "$ARM" "      - uses: $BUILD_PUSH
      - env:
          TOKEN: \${{ secrets['HCLOUD_TOKEN'] }}
        run: ./deploy")" "image-build job must reference no secret beyond the registry-push GITHUB_TOKEN"
# RED: a job key with a trailing comment defeats the job-header parser; an
# image build must then fail closed rather than silently skip classification.
assert "annotated job key with image build fails" 1 "$(printf 'jobs:\n  x: # image build\n    runs-on: %s\n    steps:\n      - uses: %s' "$ARM" "$BUILD_PUSH")" \
  "outside a recognized job header"
# GREEN: deploy-class secrets remain legal in a non-build job: the secretless
# rule keys on the image build.
assert "deploy secret in non-build job passes" 0 "$(job_body "$ARM" "      - env:
          TOKEN: \${{ secrets.PRODUCTION_TOKEN }}
        run: ./deploy")"

# --- check 6b: main-only registry publication (was the bex-build pool) --------
build_job() {
  printf 'jobs:\n  x:\n    if: %s\n    runs-on: %s\n    permissions:\n      contents: read\n      packages: write\n      id-token: write\n%b    steps:\n%s' \
    "${1:-github.ref == 'refs/heads/main'}" "$ARM" "${2:-}" "${3:-      - uses: $PINNED}"
}
# GREEN: a main-gated job with GITHUB_TOKEN write scopes and secrets.GITHUB_TOKEN.
assert "main-only GITHUB_TOKEN publisher passes" 0 "$(build_job "" "" "      - env:
          GHCR_TOKEN: \${{ secrets.GITHUB_TOKEN }}
        run: ./publish")"
# RED: any other secret, dot or bracket form, or via the workflow header.
assert "deploy secret on a publisher fails" 1 "$(build_job "" "" "      - env:
          T: \${{ secrets.BEX_SSH_PRIVATE_KEY }}
        run: ./x")" "registry-publishing job may hold only GITHUB_TOKEN"
assert "bracket-form secret on a publisher fails" 1 "$(build_job "" "" "      - env:
          T: \${{ secrets['HCLOUD_TOKEN'] }}
        run: ./x")" "registry-publishing job may hold only GITHUB_TOKEN"
assert "workflow-header secret reaching a publisher fails" 1 "$(printf 'env:\n  T: ${{ secrets.HCLOUD_TOKEN }}\n%s' "$(build_job)")" \
  "registry-publishing job may hold only GITHUB_TOKEN"
assert "toJSON(secrets) on a publisher fails" 1 "$(build_job "" "" "      - env:
          ALL: \${{ toJSON(secrets) }}
        run: ./x")" "registry-publishing job may hold only GITHUB_TOKEN"
assert "lower-case github_token on a publisher passes" 0 "$(build_job "" "" "      - env:
          T: \${{ secrets.github_token }}
        run: ./publish")"
# RED: write-all, or a job not gated to main.
assert "write-all on a publisher fails" 1 "$(printf 'permissions: write-all\n%s' "$(build_job)")" \
  "registry-publishing job must not use write-all"
assert "ungated publisher fails" 1 "$(build_job "always()")" \
  "registry-publishing job must be gated to refs/heads/main"
assert "main lookalike gate fails" 1 "$(build_job "github.ref == 'refs/heads/main-x'")" \
  "registry-publishing job must be gated to refs/heads/main"
assert "ORed main gate fails" 1 "$(build_job "github.ref == 'refs/heads/main' || true")" \
  "registry-publishing job must be gated to refs/heads/main"
assert "negated term beside the main gate fails" 1 "$(build_job "github.ref == 'refs/heads/main' && !cancelled()")" \
  "registry-publishing job must be gated to refs/heads/main"
assert "main gate only in a comment fails" 1 "$(build_job "always()" "    # if: github.ref == 'refs/heads/main'\n")" \
  "registry-publishing job must be gated to refs/heads/main"
# RED: workflow-level packages: write reaches a job without its own permissions.
assert "workflow-level packages write on an ungated job fails" 1 "$(printf 'permissions:\n  contents: read\n  packages: write\njobs:\n  x:\n    runs-on: %s\n    steps:\n      - uses: %s' "$ARM" "$PINNED")" \
  "registry-publishing job must be gated to refs/heads/main"
# GREEN: ANDed gates, either side, including a != comparison (deploy.yml build).
assert "ANDed main gate passes" 0 "$(build_job "github.ref == 'refs/heads/main' && needs.plan.outputs.ok == 'true'")"
assert "deploy build gate shape passes" 0 "$(build_job "needs.changes.outputs.deploy == 'true' && github.ref == 'refs/heads/main'")"
# GREEN: an environment-gated publisher is a production job (checks 8/9).
assert "environment-gated publisher passes" 0 "$(build_job "github.ref == 'refs/heads/main'" "    environment: production-deploy\n" "      - env:
          T: \${{ secrets.HCLOUD_TOKEN }}
        run: ./deploy")"
# RED: a PR workflow must keep PR events off a publisher (check 6a).
assert "publisher in a PR workflow fails" 1 "$(printf 'on:\n  pull_request:\n%s' "$(build_job)")" \
  "credential-bearing job must reject pull_request events"
assert "caller passing secrets to a publishing workflow fails" 1 "$(printf '%s\n  caller:\n    uses: ./.github/workflows/ci.yml\n    secrets: inherit\n' "$(build_job)")" \
  "which publishes to the registry"

# Checkout size (w6/m157): skip /.pm/ or name why the job needs it.
CHECKOUT='actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1 # v7.0.1'
assert "full-tree checkout fails" 1 "$(job_body "$ARM" "      - uses: $CHECKOUT")" \
  "actions/checkout must use a sparse-checkout that skips /.pm/"
assert "sparse checkout skipping .pm passes" 0 "$(job_body "$ARM" "      - uses: $CHECKOUT
        with:
          filter: blob:none
          sparse-checkout-cone-mode: false
          sparse-checkout: |
            /*
            !/.pm/")"
assert "named full checkout passes" 0 "$(job_body "$ARM" "      # full-checkout: gitleaks scans every tracked byte
      - uses: $CHECKOUT")"
assert "full-checkout without a reason fails" 1 "$(job_body "$ARM" "      # full-checkout:
      - uses: $CHECKOUT")" "actions/checkout must use a sparse-checkout"

# --- check 7: public-fork isolation for every PR-reachable job ---------------
pr_job_body() {
  printf 'on:\n  pull_request:\njobs:\n  x:\n    %s\n    runs-on: %s\n    steps:\n      - uses: %s' "$1" "$ARM" "$PINNED"
}
# RED: a pull_request job without a repository-identity gate runs fork code
# in this repository's CI (cache scope, runner minutes, logs).
assert "unguarded fork PR job fails" 1 "$(pr_job_body "name: unguarded")" \
  "reject public fork heads"
# GREEN: same-repository PR branches continue to run, while fork heads skip.
assert "same-repository PR guard passes" 0 "$(pr_job_body "if: github.event_name != 'pull_request' || github.event.pull_request.head.repo.full_name == github.repository")"
# GREEN: a job in a mixed-event workflow may instead exclude all PR events.
assert "PR-excluded job passes" 0 "$(pr_job_body "if: github.event_name != 'pull_request'")"

# --- check 10: bounded uncredentialed jobs (opt-in for fixtures) --------------
export BOUNDED_JOBS_CHECK=1
assert "uncredentialed job without a timeout fails" 1 "$(job_body "$ARM" "      - uses: $PINNED")" \
  "ci.yml:x"
assert "uncredentialed job with a timeout passes" 0 "$(printf 'jobs:\n  x:\n    runs-on: %s\n    timeout-minutes: 10\n    steps:\n      - uses: %s' "$ARM" "$PINNED")"
assert "secret-holding job without a timeout passes" 0 "$(job_body "$ARM" "      - env:
          T: \${{ secrets.PRODUCTION_TOKEN }}
        run: ./deploy")"
assert "environment job without a timeout passes" 0 "$(printf 'jobs:\n  x:\n    runs-on: %s\n    environment: production-observe\n    steps:\n      - uses: %s' "$ARM" "$PINNED")"
assert "write-token job without a timeout passes" 0 "$(printf 'jobs:\n  x:\n    if: github.ref == '"'"'refs/heads/main'"'"'\n    runs-on: %s\n    permissions:\n      issues: write\n    steps:\n      - uses: %s' "$ARM" "$PINNED")"
assert "second job is checked on its own" 1 "$(printf 'jobs:\n  a:\n    runs-on: %s\n    environment: production-observe\n    steps:\n      - uses: %s\n  b:\n    runs-on: %s\n    steps:\n      - uses: %s' "$ARM" "$PINNED" "$ARM" "$PINNED")" \
  "ci.yml:b"
unset BOUNDED_JOBS_CHECK

# --- w1/m68 F3: host-key pin coverage for admin.conf fetchers ---------------
# RED: a workflow that fetches admin.conf over SSH without wiring the pin. This
# is exactly the shape openbao-restore-drill.yml had — the whole reason m66's
# fix was incomplete — so it is the case that must be proven red.
assert "unpinned admin.conf fetcher fails" 1 "$(body "      - uses: $PINNED
      - run: bash scripts/fetch-app-kubeconfig.sh \"\$RUNNER_TEMP/app.kubeconfig\"")" \
  "trust the control-plane host key on first use"
# RED: the same for the substrate verifier, the other SSH entry point.
assert "unpinned verify-substrate fails" 1 "$(body "      - uses: $PINNED
      - run: bash scripts/verify-substrate.sh")" \
  "trust the control-plane host key on first use"
# GREEN: wiring the secret satisfies it.
assert "pinned admin.conf fetcher passes" 0 "$(body "      - uses: $PINNED
      - env:
          BEX_SSH_KNOWN_HOSTS: \${{ secrets.BEX_SSH_KNOWN_HOSTS }}
        run: |
          bash scripts/fetch-app-kubeconfig.sh \"\$RUNNER_TEMP/app.kubeconfig\"
          kubectl get nodes
          rm -f -- \"\$RUNNER_TEMP/app.kubeconfig\"")"
# GREEN: a workflow that never fetches admin.conf is unaffected.
assert "non-fetcher unaffected" 0 "$(body "      - uses: $PINNED
      - run: make test")"

# RED: scrubbing the fetched kubeconfig before cluster work reproduces deploy
# run 32913749053's localhost:8080 failure.
assert "premature kubeconfig scrub fails" 1 "$(body "      - uses: $PINNED
      - env:
          BEX_SSH_KNOWN_HOSTS: \${{ secrets.BEX_SSH_KNOWN_HOSTS }}
        run: |
          bash scripts/fetch-app-kubeconfig.sh \"\$RUNNER_TEMP/app.kubeconfig\"
          rm -f -- \"\$RUNNER_TEMP/app.kubeconfig\"
      - run: kubectl get nodes")" \
  "delete app.kubeconfig before their last cluster command"
# RED: an early scrub plus a final scrub is still broken during the workload.
assert "duplicate kubeconfig scrub fails" 1 "$(body "      - uses: $PINNED
      - env:
          BEX_SSH_KNOWN_HOSTS: \${{ secrets.BEX_SSH_KNOWN_HOSTS }}
        run: |
          bash scripts/fetch-app-kubeconfig.sh \"\$RUNNER_TEMP/app.kubeconfig\"
          rm -f -- \"\$RUNNER_TEMP/app.kubeconfig\"
      - run: kubectl get nodes
      - run: rm -f -- \"\$RUNNER_TEMP/app.kubeconfig\"")" \
  "do not scrub it exactly once"

# --- w8/m30 t005 / ADR050: restore workflows must carry the age decrypt key --
# RED: a restore workflow that disables dotenv but never passes the key. This is
# exactly what failed run 32814333448 — the drill's env list predated Tier A
# encryption — so it is the case that must be proven red.
assert "keyless restore workflow fails" 1 "$(body "      - uses: $PINNED
      - env:
          RESTORE_SKIP_DOTENV: \"1\"
        run: bash scripts/restore-openbao.sh --target-namespace restore-x --verify-path p")" \
  "AGE_BACKUP_PRIVATE_KEY"
# RED: the key alone is insufficient when the runner has no age executable.
assert "keyed restore without age runtime fails" 1 "$(body "      - uses: $PINNED
      - env:
          RESTORE_SKIP_DOTENV: \"1\"
          AGE_BACKUP_PRIVATE_KEY: \${{ secrets.AGE_BACKUP_PRIVATE_KEY }}
        run: bash scripts/restore-openbao.sh --target-namespace restore-x --verify-path p")" \
  "RESTORE_AGE_IMAGE"
# GREEN: a digest-pinned fallback image gives the restore helper an age runtime.
assert "keyed restore with pinned age runtime passes" 0 "$(body "      - uses: $PINNED
      - env:
          RESTORE_SKIP_DOTENV: \"1\"
          AGE_BACKUP_PRIVATE_KEY: \${{ secrets.AGE_BACKUP_PRIVATE_KEY }}
          RESTORE_AGE_IMAGE: example.invalid/age@sha256:0000000000000000000000000000000000000000000000000000000000000000
        run: bash scripts/restore-openbao.sh --target-namespace restore-x --verify-path p")"
# GREEN: a restore invocation that keeps dotenv loading gets the key from .env.
assert "dotenv restore unaffected" 0 "$(body "      - uses: $PINNED
      - run: bash scripts/restore-etcd.sh")"

# GREEN: the real canonical tree passes end-to-end (inventory + deploy wiring).
if "$SCRIPT" >/dev/null 2>&1; then
  echo "ok: real tree passes"
else
  echo "FAIL: real tree — validator red on the canonical .github/workflows" >&2
  fails=$((fails + 1))
fi

# Durable structural guards for the sibling deliverables (m58 "can't be silently
# deleted" spirit): the dashboard audit gate and the dependabot config.
dash="$root/.github/workflows/dashboard-test.yml"
crit='yarn npm audit .*--severity[= ]critical'
# Present + severity intact (deletion / critical→high downgrade turns this red)
# AND the gate isn't neutered inline with `|| true` (a weakening this catches).
if grep -Eq "$crit" "$dash" \
  && grep -Eq 'yarn npm audit .*--severity[= ]high' "$dash" \
  && ! grep -E "$crit" "$dash" | grep -q '|| true'; then
  echo "ok: dashboard audit gate present (critical gate + high warn, gate not neutered)"
else
  echo "FAIL: dashboard-test.yml lost/neutered its yarn npm audit critical-gate + high-warn steps" >&2
  fails=$((fails + 1))
fi

# --- check 9: production-secret confinement (w3/m173, #ENV-REVIEWERS) ---------
secrets_table="$tmp/production-secrets.txt"
printf '%s\n' '# fixture table' 'PROD_TOKEN production-deploy,production-observe' >"$secrets_table"
prod_job() { # <environment-line-or-empty> <secret-name>
  printf 'jobs:\n  x:\n    runs-on: ubuntu-24.04-arm\n%s    steps:\n      - run: echo ok\n        env:\n          T: ${{ secrets.%s }}' "$1" "$2"
}
export PRODUCTION_SECRETS_FILE="$secrets_table"
# GREEN: the secret is read inside one of its assigned environments.
assert "production secret in assigned environment passes" 0 "$(prod_job '    environment: production-observe
' PROD_TOKEN)"
# GREEN: the mapping form (environment: {name: …}) is understood.
assert "production secret in mapping-form environment passes" 0 "$(prod_job '    environment:
      name: production-deploy
' PROD_TOKEN)"
# GREEN: a secret absent from the table is not this check's business.
assert "unlisted secret passes" 0 "$(prod_job '' OTHER_TOKEN)"
# RED: no environment at all.
assert "production secret without environment fails" 1 "$(prod_job '' PROD_TOKEN)" \
  "reads production secret PROD_TOKEN outside a production environment"
# RED: an environment the table does not assign to that secret.
assert "production secret in wrong environment fails" 1 "$(prod_job '    environment: production-release
' PROD_TOKEN)" "in environment production-release"
# RED: workflow-level env exposes the secret to every job.
assert "workflow-level production secret fails" 1 "$(printf 'env:\n  T: ${{ secrets.PROD_TOKEN }}\n%s' "$(prod_job '    environment: production-deploy
' OTHER_TOKEN)")" "above jobs:"
# GREEN: a commented-out reference is not a read.
assert "commented production secret ignored" 0 "$(printf 'jobs:\n  x:\n    runs-on: ubuntu-24.04-arm\n    steps:\n      # T: ${{ secrets.PROD_TOKEN }}\n      - run: echo ok')"
# RED: a commented job header must not hand its secrets to the previous job's environment.
assert "commented job header does not inherit the previous environment" 1 "$(printf 'jobs:\n  a:\n    runs-on: ubuntu-24.04-arm\n    environment: production-deploy\n    steps:\n      - run: echo ok\n  b: # probe\n    runs-on: ubuntu-24.04-arm\n    steps:\n      - run: echo ok\n        env:\n          T: ${{ secrets.PROD_TOKEN }}')" \
  "b: reads production secret PROD_TOKEN outside a production environment"
# GREEN: mapping form with a trailing comment on `environment:`.
assert "commented mapping-form environment passes" 0 "$(prod_job '    environment: # gate
      name: production-observe
' PROD_TOKEN)"
# GREEN: flow-mapping form.
assert "flow-mapping environment passes" 0 "$(prod_job '    environment: { name: production-deploy, url: https://bex.co }
' PROD_TOKEN)"
unset PRODUCTION_SECRETS_FILE

# Scan the patch actually present in the immutable release image, including
# when its readable tag names only the major/minor line. A latest-release
# lookup would pass over vulnerabilities still embedded in the pinned image.
go_fixture="$tmp/release-go"
mkdir -p "$go_fixture/bin" "$go_fixture/lego"
resolver=$(yq -r '.jobs.govulncheck.steps[] | select(.id == "goline") | .run' "$root/.github/workflows/govulncheck.yml")
cat >"$go_fixture/bin/docker" <<'EOF'
#!/usr/bin/env bash
printf '%s\n' "$@" >"$DOCKER_ARGS"
printf '%s\n' "$IMAGE_GO_VERSION"
EOF
chmod +x "$go_fixture/bin/docker"
release_image="golang:1.26@sha256:$(printf '%064d' 1)"
# The real builder line carries --platform=$BUILDPLATFORM (w6/m158).
printf 'FROM --platform=$BUILDPLATFORM %s AS builder\n' "$release_image" >"$go_fixture/lego/Dockerfile"
if (cd "$go_fixture" && PATH="$go_fixture/bin:$PATH" \
    DOCKER_ARGS="$go_fixture/docker-args" IMAGE_GO_VERSION=go1.26.8 \
    GITHUB_OUTPUT="$go_fixture/output" bash -euo pipefail -c "$resolver") \
  && grep -qx 'version=1.26.8' "$go_fixture/output" \
  && grep -qxF "$release_image" "$go_fixture/docker-args" \
  && [ "$(yq -r '.jobs.govulncheck.steps[] | select(.name == "Scan the Go platform modules (fail on any reachable vuln)") | .env.GOTOOLCHAIN' "$root/.github/workflows/govulncheck.yml")" = local ]; then
  echo "ok: govulncheck scans the pinned image's patch without toolchain auto-upgrades"
else
  echo "FAIL: govulncheck toolchain differs from the release image" >&2
  fails=$((fails + 1))
fi
for bad in unpinned malformed-version; do
  printf 'FROM %s AS builder\n' "$release_image" >"$go_fixture/lego/Dockerfile"
  image_version=not-a-go-version
  if [ "$bad" = unpinned ]; then
    printf 'FROM golang:1.26 AS builder\n' >"$go_fixture/lego/Dockerfile"
    image_version=go1.26.8
  fi
  if (cd "$go_fixture" && PATH="$go_fixture/bin:$PATH" \
      DOCKER_ARGS="$go_fixture/docker-args" IMAGE_GO_VERSION="$image_version" \
      GITHUB_OUTPUT="$go_fixture/output" bash -euo pipefail -c "$resolver") >"$go_fixture/error" 2>&1; then
    echo "FAIL: release Go resolver accepted $bad" >&2
    fails=$((fails + 1))
  else
    echo "ok: release Go resolver rejects $bad"
  fi
done

dependabot="$root/.github/dependabot.yml"
# Every Go module that ships builds standalone (GOWORK=off), so each needs its
# own Dependabot directory; one missing means its advisories never get a PR
# (w1/m174: reachable x/net advisories sat red for six days with none).
go_dirs="$(yq -N '[.updates[] | select(."package-ecosystem" == "gomod") | .directories[]] | sort | join(",")' "$dependabot" 2>/dev/null || true)"
if grep -q 'package-ecosystem: *"\?github-actions' "$dependabot" 2>/dev/null \
  && grep -q 'package-ecosystem: *"\?npm' "$dependabot" 2>/dev/null \
  && [ "$go_dirs" = "/lego/backend,/lego/cli,/lego/operator,/lego/types" ]; then
  echo "ok: dependabot.yml declares github-actions, npm and every lego/ Go module"
else
  echo "FAIL: .github/dependabot.yml missing github-actions, npm, or a lego/ Go module (gomod directories: '$go_dirs')" >&2
  fails=$((fails + 1))
fi

if [ "$fails" -eq 0 ]; then
  echo "PASS: github-actions-validate.sh"
else
  echo "FAIL: $fails case(s)" >&2
  exit 1
fi
