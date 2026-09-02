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

PINNED='actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1 # v7.0.1'
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

# --- ADR083 / #CI-RUNNERS: self-hosted runner custody -----------------------
job_body() {
  printf 'jobs:\n  x:\n    runs-on:\n      group: %s\n      labels: [self-hosted, Linux, ARM64]\n    steps:\n%s' "$1" "$2"
}
# RED: GitHub-hosted ubuntu-latest is a rejected remediation.
assert "ubuntu-latest fails" 1 "$(printf 'jobs:\n  x:\n    runs-on: ubuntu-latest\n    steps:\n      - uses: %s' "$PINNED")" \
  "GitHub-hosted ubuntu runners"
# RED: the legacy shared label pool no longer passes.
assert "shared self-hosted pool fails" 1 "$(printf 'jobs:\n  x:\n    runs-on: [self-hosted, Linux, ARM64]\n    steps:\n      - uses: %s' "$PINNED")" \
  "must select exactly one approved self-hosted runner group"
# RED: an arbitrary runner group cannot silently create a third trust class.
assert "unknown runner group fails" 1 "$(job_body "default" "      - uses: $PINNED")" \
  "missing approved bex-ci or bex-production group"
# GREEN: both canonical self-hosted groups pass the structural contract.
assert "CI runner group passes" 0 "$(job_body "bex-ci" "      - uses: $PINNED")"
assert "production runner group passes" 0 "$(job_body "bex-production" "      - uses: $PINNED")"
# RED: the runner account deliberately has no sudo; tools belong in RUNNER_TEMP.
assert "sudo workflow fails" 1 "$(job_body "bex-ci" "      - uses: $PINNED
      - run: sudo apt-get install shellcheck")" "must not require sudo"

# RED: secrets and write-capable tokens must never land on the PR-capable pool.
assert "secret on CI runner fails" 1 "$(job_body "bex-ci" "      - env:
          TOKEN: \${{ secrets.PRODUCTION_TOKEN }}
        run: ./deploy")" "credential-bearing job must use bex-production"
assert "write token on CI runner fails" 1 "$(printf 'jobs:\n  x:\n    runs-on:\n      group: bex-ci\n      labels: [self-hosted, Linux, ARM64]\n    permissions:\n      contents: write\n    steps:\n      - uses: %s' "$PINNED")" \
  "credential-bearing job must use bex-production"
assert "write-all token on CI runner fails" 1 "$(printf 'permissions: write-all\njobs:\n  x:\n    runs-on:\n      group: bex-ci\n      labels: [self-hosted, Linux, ARM64]\n    steps:\n      - uses: %s' "$PINNED")" \
  "credential-bearing job must use bex-production"
assert "environment on CI runner fails" 1 "$(printf 'jobs:\n  x:\n    runs-on:\n      group: bex-ci\n      labels: [self-hosted, Linux, ARM64]\n    environment: production-deploy\n    steps:\n      - uses: %s' "$PINNED")" \
  "credential-bearing job must use bex-production"
# GREEN: the same credential material is allowed only on the production pool.
assert "secret on production runner passes" 0 "$(job_body "bex-production" "      - env:
          TOKEN: \${{ secrets.PRODUCTION_TOKEN }}
        run: ./deploy")"

# --- Public-fork isolation --------------------------------------------------
pr_job_body() {
  printf 'on:\n  pull_request:\njobs:\n  x:\n    %s\n    runs-on:\n      group: %s\n      labels: [self-hosted, Linux, ARM64]\n    steps:\n      - uses: %s' "$1" "${2:-bex-ci}" "$PINNED"
}
# RED: a pull_request job without a repository-identity gate can schedule fork
# code on a persistent self-hosted runner.
assert "unguarded fork PR job fails" 1 "$(pr_job_body "name: unguarded")" \
  "reject public fork heads"
# GREEN: same-repository PR branches continue to run, while fork heads skip.
assert "same-repository PR guard passes" 0 "$(pr_job_body "if: github.event_name != 'pull_request' || github.event.pull_request.head.repo.full_name == github.repository")"
# GREEN: a job in a mixed-event workflow may instead exclude all PR events.
assert "PR-excluded self-hosted job passes" 0 "$(pr_job_body "if: github.event_name != 'pull_request'")"
# RED: even a same-repository gate cannot schedule PR work onto production.
assert "same-repository PR on production runner fails" 1 "$(pr_job_body "if: github.event.pull_request.head.repo.full_name == github.repository" "bex-production")" \
  "bex-production job must reject pull_request events"
# GREEN: a production job may live in a mixed-event workflow only when it
# rejects pull_request before scheduling (the infra.yml terraform shape).
assert "PR-excluded production job passes" 0 "$(pr_job_body "if: github.event_name != 'pull_request'" "bex-production")"

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

dependabot="$root/.github/dependabot.yml"
if grep -q 'package-ecosystem: *"\?github-actions' "$dependabot" 2>/dev/null \
  && grep -q 'package-ecosystem: *"\?npm' "$dependabot" 2>/dev/null; then
  echo "ok: dependabot.yml declares github-actions + npm ecosystems"
else
  echo "FAIL: .github/dependabot.yml missing github-actions and/or npm ecosystem" >&2
  fails=$((fails + 1))
fi

if [ "$fails" -eq 0 ]; then
  echo "PASS: github-actions-validate.sh"
else
  echo "FAIL: $fails case(s)" >&2
  exit 1
fi
