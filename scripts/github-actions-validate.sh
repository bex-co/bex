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
#  4. Workflows fetching admin.conf pin the SSH host and keep the fetched
#     kubeconfig alive until their last cluster command.
#  5. GitHub-hosted runners (owner decision 2026-11-09; ADR083 amendment,
#     `.pm/DO_NOT_DO.md` #CI-RUNNERS re-opened): every job `runs-on` is the
#     scalar `ubuntu-24.04-arm` (default) or `ubuntu-latest` (x64, for
#     linux/amd64 image work only), and no workflow mentions `self-hosted`.
#  6. Credential boundary: a credential-bearing job (secrets, environment or a
#     write token) in a `pull_request` workflow must reject PR events, and a
#     registry-publishing job without an environment is main-only and holds no
#     secret beyond GITHUB_TOKEN (what the retired bex-build pool enforced).
#  7. Public-fork isolation: any job in a `pull_request` workflow must either
#     reject fork PRs by repository identity or reject all PR events.
#  8. Secretless image build (ADR080 F2, w2/m89): a job that builds images with
#     docker/build-push-action executes repo-controlled code, so it may
#     reference no secret beyond the registry-push GITHUB_TOKEN and must not
#     carry an `environment:` gate — deploy credentials belong to a separate
#     environment-gated job consuming the build's digests.
#  9. Production-secret confinement (w3/m173, ADR083 obligation 5,
#     `.pm/DO_NOT_DO.md` #ENV-REVIEWERS): a job may read a secret listed in
#     scripts/production-secrets.txt only while running in one of the
#     environments that table assigns to it, and no workflow may reference one
#     above `jobs:` (where it would reach every job). With every production-*
#     environment main-only and the secrets in environment scope, a workflow on
#     any other branch cannot obtain them.
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

collect_workflow_files() {
  if [ "$canonical_tree" -eq 1 ]; then
    find .github/workflows lego/operator/.github/workflows -name '*.yml' 2>/dev/null | LC_ALL=C sort
  else
    find "$WORKFLOWS_DIR" -name '*.yml' 2>/dev/null | LC_ALL=C sort
  fi
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

# 3b. One tracking-issue path (w6/m161). Every scheduled monitor opens, updates
# and closes its issue through scripts/lib/tracking-issue.sh, which retries a
# lost create only after re-listing; a hand-rolled `gh issue create` or
# `gh issue edit` can open a duplicate or skip the label.
direct_issue_writes="$(grep -HnE 'gh issue (create|edit)([^[:alnum:]_-]|$)' "$WORKFLOWS_DIR"/*.yml 2>/dev/null \
  | grep -vE '^[^:]+:[0-9]+:[[:space:]]*#' | cut -d: -f1,2 || true)"
if [ -n "$direct_issue_writes" ]; then
  echo "FAIL: workflows must open/update tracking issues with scripts/lib/tracking-issue.sh, not gh issue create/edit:" >&2
  printf '  %s\n' $direct_issue_writes >&2
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

# 4a. Kubeconfig credential lifetime. The SSH key can and should be scrubbed as
# soon as admin.conf has been fetched, but deleting app.kubeconfig in that same
# step leaves KUBECONFIG pointing at a missing file. kubectl then falls back to
# localhost:8080 and every real cluster operation fails. Derived from the same
# fetcher inventory as the host-key check so new credentialed workflows inherit
# the guard automatically.
invalid_kubeconfig_lifetimes=""
for wf in $(admin_conf_fetchers); do
  cleanup_count="$(grep -cF 'rm -f -- "$RUNNER_TEMP/app.kubeconfig"' "$wf" || true)"
  cleanup_line="$(grep -nF 'rm -f -- "$RUNNER_TEMP/app.kubeconfig"' "$wf" \
    | tail -n 1 | cut -d: -f1 || true)"
  last_cluster_line="$(grep -nE '(^|[^[:alnum:]_-])(kubectl|helm|clusterctl)([^[:alnum:]_-]|$)' "$wf" \
    | grep -vE '^[0-9]+:[[:space:]]*#' | tail -n 1 | cut -d: -f1 || true)"
  if [ "$cleanup_count" -ne 1 ] \
    || { [ -n "$last_cluster_line" ] && [ "${cleanup_line:-0}" -le "$last_cluster_line" ]; }; then
    invalid_kubeconfig_lifetimes="$invalid_kubeconfig_lifetimes $wf"
  fi
done
if [ -n "$invalid_kubeconfig_lifetimes" ]; then
  echo "FAIL: these workflows delete app.kubeconfig before their last cluster command (or do not scrub it exactly once):" >&2
  printf '  %s\n' $invalid_kubeconfig_lifetimes >&2
  exit 1
fi

# 4b. ADR050 Tier A decrypt custody. A workflow that runs a
# scripts/restore-*.sh with dotenv loading disabled supplies the script's whole
# environment itself — and since 2026-08-04 every Tier A snapshot is
# age-encrypted, so omitting AGE_BACKUP_PRIVATE_KEY fails at the decrypt step.
# Derived from the tree so a future etcd/KeyValue restore workflow is caught
# the day it is added.
restore_workflows() {
  local workflow
  while IFS= read -r workflow; do
    grep -lE 'scripts/restore-[a-z]+\.sh' "$workflow" 2>/dev/null || true
  done < <(collect_workflow_files)
}
restore_workflow_files=()
restore_workflow_count=0
while IFS= read -r wf; do
  if [ -n "$wf" ]; then
    restore_workflow_files+=("$wf")
    restore_workflow_count=$((restore_workflow_count + 1))
  fi
done < <(restore_workflows)
keyless_restores=()
ageless_restores=()
keyless_restore_count=0
ageless_restore_count=0
if [ "$restore_workflow_count" -gt 0 ]; then
  for wf in "${restore_workflow_files[@]}"; do
    grep -Fq 'RESTORE_SKIP_DOTENV' "$wf" || continue
    if ! grep -Fq 'AGE_BACKUP_PRIVATE_KEY' "$wf"; then
      keyless_restores+=("$wf")
      keyless_restore_count=$((keyless_restore_count + 1))
    fi
    if ! grep -Eq '^[[:space:]]+RESTORE_AGE_IMAGE:[[:space:]]+[^[:space:]#]+@sha256:[0-9a-f]{64}([[:space:]#]|$)' "$wf"; then
      if ! grep -Fq 'AGE_LINUX_ARM64_SHA256: c6878a324421b69e3e20b00ba17c04bc5c6dab0030cfe55bf8f68fa8d9e9093a' "$wf" \
        || ! grep -Fq 'age-v${AGE_VERSION}-linux-arm64.tar.gz' "$wf" \
        || ! grep -Fq 'sha256sum --check --strict' "$wf" \
        || ! grep -Fq '>>"$GITHUB_PATH"' "$wf"; then
        ageless_restores+=("$wf")
        ageless_restore_count=$((ageless_restore_count + 1))
      fi
    fi
  done
fi
if [ "$keyless_restore_count" -gt 0 ]; then
  echo "FAIL: these workflows run a restore script with RESTORE_SKIP_DOTENV but never pass" >&2
  echo "      AGE_BACKUP_PRIVATE_KEY, so an .age snapshot fails at the decrypt step (ADR050):" >&2
  printf '  %s\n' "${keyless_restores[@]}" >&2
  exit 1
fi
if [ "$ageless_restore_count" -gt 0 ]; then
  echo "FAIL: these workflows can receive an encrypted restore but provide neither" >&2
  echo "      a digest-pinned RESTORE_AGE_IMAGE nor a checksum-pinned age CLI:" >&2
  printf '  %s\n' "${ageless_restores[@]}" >&2
  exit 1
fi

# 5. GitHub-hosted runners (owner decision 2026-11-09, re-opening
# .pm/DO_NOT_DO.md #CI-RUNNERS; ADR083 amendment). Every job runs on a fresh
# GitHub-hosted VM that is discarded afterwards, so no job inherits another
# job's files, tools or credentials. `runs-on` is exactly one scalar label:
# `ubuntu-24.04-arm` by default (native ARM64, so the pinned *_linux_arm64
# tool downloads and scripts/hashicorp-install.sh stay valid) or
# `ubuntu-latest` (x64) for linux/amd64 image work that would otherwise run
# under QEMU. Matrix/expression labels, runner groups and every `self-hosted`
# mention fail: the operator-custodied pools are retired, and a stale label
# would queue forever rather than run.
invalid_runner_contract=""
self_hosted_mentions=""
for wf in $(collect_workflow_files); do
  mentions="$(grep -n 'self-hosted' "$wf" | cut -d: -f1 | sed "s|^|$wf:|" || true)"
  [ -z "$mentions" ] || self_hosted_mentions="${self_hosted_mentions}${self_hosted_mentions:+
}${mentions}"
  violations="$(awk -v file="$wf" '
    /^[[:space:]]*runs-on:/ {
      if ($0 ~ /^    runs-on: (ubuntu-24\.04-arm|ubuntu-latest)[[:space:]]*$/) next
      print file ":" NR ": runs-on must be the scalar ubuntu-24.04-arm (default) or ubuntu-latest (amd64 image work)"
    }
  ' "$wf")"
  [ -z "$violations" ] || invalid_runner_contract="${invalid_runner_contract}${invalid_runner_contract:+
}${violations}"
done
if [ -n "$self_hosted_mentions" ]; then
  echo "FAIL: workflows must not mention self-hosted runners — bex CI runs on GitHub-hosted runners (ADR083 amendment 2026-11-09, .pm/DO_NOT_DO.md #CI-RUNNERS):" >&2
  printf '  %s\n' $self_hosted_mentions >&2
  exit 1
fi
if [ -n "$invalid_runner_contract" ]; then
  echo "FAIL: every job must run on exactly one approved GitHub-hosted runner label:" >&2
  printf '%s\n' "$invalid_runner_contract" >&2
  exit 1
fi

# 6. Credential boundary, keyed on what the retired trust pools signaled.
#
# 6a (was "bex-ci holds no credential; bex-production rejects PRs"): a job that
# can run for a pull_request must hold nothing worth stealing. A job holding
# repository/environment secrets, an `environment:`, write-all or any write
# token scope, in a workflow a pull_request(_target) triggers, must reject
# that event before GitHub schedules it (the infra.yml terraform shape). A
# top-level secret or write permission applies to every job in the workflow.
# The rejection must be an ANDed `github.event_name != 'pull_request'` on the
# job's `if:`; the fork guard's `… || same repository` form still admits PRs.
#
# 6b (was the bex-build pool, w6/m155): a registry-publishing job
# (`packages: write`, job- or workflow-level) without an environment is
# main-only image publication. It may hold GITHUB_TOKEN write scopes but no
# other secret and not write-all, and its `if:` must AND in
# github.ref == 'refs/heads/main' (no `||`, no negation). Environment-gated
# publishers (the deploy job) are production jobs instead: checks 8 and 9.
runner_trust_violations() {
  local wf
  for wf in $(collect_workflow_files); do
    awk -v file="$wf" '
      function check_job() {
        if (job == "" || block !~ /\n    runs-on:/) return
        foreign_secret = workflow_foreign_secret || block_foreign_secret(block)
        environment = block ~ /\n    environment:/
        job_write_all = block ~ /\n    permissions:[[:space:]]*write-all([[:space:]]|$)/
        credentialed = workflow_credentialed \
          || block ~ /\$\{\{[[:space:]]*secrets\./ \
          || whole_secrets(block) \
          || environment || job_write_all \
          || block ~ /\n      [A-Za-z0-9_-]+:[[:space:]]*write([[:space:]]|$)/
        own_permissions = block ~ /\n    permissions:/
        publishes = block ~ /\n      packages:[[:space:]]*write([[:space:]]|$)/ \
          || job_write_all || (!own_permissions && (workflow_packages_write || workflow_write_all))
        if (credentialed && ((has_pull_request && !rejects_event(block, "pull_request")) \
          || (has_pull_request_target && !rejects_event(block, "pull_request_target")))) {
          print file ":" job ": credential-bearing job must reject pull_request events"
        }
        if (publishes && !environment) {
          if (foreign_secret) print file ":" job ": registry-publishing job may hold only GITHUB_TOKEN, not another secret"
          if (workflow_write_all || job_write_all) print file ":" job ": registry-publishing job must not use write-all"
          if (!main_gated(block)) print file ":" job ": registry-publishing job must be gated to refs/heads/main"
        }
      }
      # The job-level single-line `if:` ANDs in `github.event_name != <event>`.
      # An `||` (as in the fork guard "!= pull_request || same repository")
      # would still schedule same-repository PRs, so it does not count.
      function rejects_event(text, event,    cond) {
        if (!match(text, /\n    if: [^\n]*/)) return 0
        cond = substr(text, RSTART + 9, RLENGTH - 9)
        if (cond ~ /\|\|/) return 0
        return index(cond, "github.event_name != \047" event "\047") > 0
      }
      # The job-level single-line `if:` ANDs in the exact main ref; any `||`
      # or negation (other than a `!=` comparison) could re-open other refs.
      function main_gated(text,    cond) {
        if (!match(text, /\n    if: [^\n]*/)) return 0
        cond = substr(text, RSTART + 9, RLENGTH - 9)
        sub(/[[:space:]]+$/, "", cond)
        if (cond ~ /\|\|/) return 0
        gsub(/!=/, "", cond)
        if (cond ~ /!/) return 0
        return cond ~ /(^|&&[[:space:]]*)github\.ref == \047refs\/heads\/main\047[[:space:]]*(&&|$)/
      }
      # The whole secrets object inside an expression, e.g. toJSON(secrets):
      # it carries every secret at once.
      function whole_secrets(text) {
        return text ~ /\$\{\{[^}]*[^A-Za-z0-9_.]secrets[[:space:]]*(\)|\}\})/
      }
      # A secrets.<NAME> reference other than GITHUB_TOKEN (dot or bracket form;
      # context names are case-insensitive), or the whole secrets object.
      function block_foreign_secret(text,    rest, name) {
        if (whole_secrets(text)) return 1
        rest = text
        while (match(rest, /secrets(\.[A-Za-z0-9_]+|\[[^]]*\])/)) {
          name = substr(rest, RSTART + 7, RLENGTH - 7)
          gsub(/^[.[]|[]"\047 ]/, "", name)
          if (toupper(name) != "GITHUB_TOKEN") return 1
          rest = substr(rest, RSTART + RLENGTH)
        }
        return 0
      }
      !in_jobs && /^  [A-Za-z0-9_-]+:[[:space:]]*write([[:space:]]|$)/ { workflow_credentialed = 1 }
      !in_jobs && /^  packages:[[:space:]]*write([[:space:]]|$)/ { workflow_packages_write = 1 }
      !in_jobs && /^permissions:[[:space:]]*write-all([[:space:]]|$)/ { workflow_credentialed = 1 }
      !in_jobs && /\$\{\{[[:space:]]*secrets\./ { workflow_credentialed = 1 }
      !in_jobs && block_foreign_secret($0) { workflow_foreign_secret = 1 }
      !in_jobs && /^permissions:[[:space:]]*write-all([[:space:]]|$)/ { workflow_write_all = 1 }
      /^  pull_request:[[:space:]]*$/ { has_pull_request = 1 }
      /^  pull_request_target:[[:space:]]*$/ { has_pull_request_target = 1 }
      /^jobs:[[:space:]]*$/ { in_jobs = 1; next }
      in_jobs && /^  [A-Za-z0-9_-]+:[[:space:]]*$/ {
        check_job()
        job = $1
        sub(/:$/, "", job)
        block = "\n" $0
        next
      }
      in_jobs && job != "" { block = block "\n" $0 }
      END { check_job() }
    ' "$wf"
  done
}

# 6c. A reusable workflow that publishes to the registry must not be handed
# secrets by its caller (`secrets:` mapping or `secrets: inherit`): the
# callee's own scan cannot see what the caller passes.
build_caller_violations() {
  local wf callee name
  for callee in $(collect_workflow_files); do
    grep -qE '^ *packages:[[:space:]]*write([[:space:]]|$)' "$callee" || continue
    name="$(basename "$callee")"
    for wf in $(collect_workflow_files); do
      awk -v file="$wf" -v callee="$name" '
        function check() {
          if (job != "" && block ~ ("\n    uses: \\./\\.github/workflows/" callee) && block ~ /\n    secrets:/)
            print file ":" job ": passes secrets to " callee ", which publishes to the registry"
        }
        /^jobs:[[:space:]]*$/ { in_jobs = 1; next }
        in_jobs && /^  [A-Za-z0-9_-]+:[[:space:]]*$/ { check(); job = $1; sub(/:$/, "", job); block = "\n" $0; next }
        in_jobs && job != "" { block = block "\n" $0 }
        END { check() }
      ' "$wf"
    done
  done
}

# 6b. Checkout size (w6/m157). .pm board evidence is ~93% of the tree, and a
# full checkout of it held production runners for 31 minutes. Every
# actions/checkout step uses a sparse checkout (skipping /.pm/ unless it re-
# includes the board paths it reads) or says why it needs everything with a
# `# full-checkout: <reason>` comment on the line above.
checkout_violations() {
  local wf
  for wf in $(collect_workflow_files); do
    # lego/operator/.github is kubebuilder scaffolding GitHub never runs. The
    # two native image workflows are byte-pinned by w8/m91's D3 custody
    # (.github/scripts/native-common-qualify.py), so they keep their checkouts
    # until that evidence is re-recorded.
    case "$wf" in
      lego/operator/.github/* | */native-base-image.yml | */native-common-image.yml) continue ;;
    esac
    awk -v file="$wf" '
      function flush() {
        if (open && !sparse && !full) print file ":" line ": actions/checkout must use a sparse-checkout that skips /.pm/, or carry a # full-checkout: <reason> comment"
        open = 0
      }
      /^[[:space:]]*(- )?uses:[[:space:]]*actions\/checkout@/ {
        flush()
        open = 1; sparse = 0; line = NR; full = (prev ~ /#[[:space:]]*full-checkout:[[:space:]]*[^[:space:]]/)
        indent = match($0, /[^ ]/)
        prev = $0; next
      }
      open {
        if ($0 ~ /^[[:space:]]*$/) { prev = $0; next }
        if (match($0, /[^ ]/) <= indent || $0 ~ /^[[:space:]]*- /) flush()
        else if ($0 ~ /sparse-checkout:/) sparse = 1
      }
      { prev = $0 }
      END { flush() }
    ' "$wf"
  done
}
checkout_problems="$(checkout_violations)"
if [ -n "$checkout_problems" ]; then
  echo "FAIL: a checkout fetches the whole tree, .pm board evidence included:" >&2
  printf '%s\n' "$checkout_problems" >&2
  exit 1
fi

trust_violations="$(runner_trust_violations; build_caller_violations)"
if [ -n "$trust_violations" ]; then
  echo "FAIL: credentials reach pull_request jobs, or registry publication escapes main:" >&2
  printf '%s\n' "$trust_violations" >&2
  exit 1
fi

# 7. Public-fork isolation. A public fork can control code reached by `make`,
# package-manager, and test steps without editing the workflow itself. Every
# job in a pull_request workflow therefore has to prove one of two
# things before GitHub schedules it: the PR head belongs to this repository, or
# the job excludes pull_request events entirely. This is defense in depth with
# the repository's "Require approval for all external contributors" setting;
# neither control substitutes for the other.
pull_request_workflows() {
  grep -lE '^  pull_request(_target)?:' "$WORKFLOWS_DIR"/*.yml 2>/dev/null || true
}

unguarded_pr_jobs() {
  local wf
  for wf in $(pull_request_workflows); do
    awk -v file="$wf" '
      function check_job() {
        if (job == "" || block !~ /\n    runs-on:/) return
        same_repository = block ~ /github\.event\.pull_request\.head\.repo\.full_name[[:space:]]*==[[:space:]]*github\.repository/
        excludes_pr = block ~ /github\.event_name[[:space:]]*!=[[:space:]]*[^[:space:]]*pull_request/
        excludes_pr_target = block ~ /github\.event_name[[:space:]]*!=[[:space:]]*[^[:space:]]*pull_request_target/
        if (!same_repository \
          && ((has_pull_request && !excludes_pr) \
            || (has_pull_request_target && !excludes_pr_target))) print file ":" job
      }
      /^  pull_request:[[:space:]]*$/ { has_pull_request = 1 }
      /^  pull_request_target:[[:space:]]*$/ { has_pull_request_target = 1 }
      /^jobs:[[:space:]]*$/ { in_jobs = 1; next }
      in_jobs && /^  [A-Za-z0-9_-]+:[[:space:]]*$/ {
        check_job()
        job = $1
        sub(/:$/, "", job)
        block = "\n" $0
        next
      }
      in_jobs && job != "" { block = block "\n" $0 }
      END { check_job() }
    ' "$wf"
  done
}

unguarded_fork_jobs="$(unguarded_pr_jobs)"
if [ -n "$unguarded_fork_jobs" ]; then
  echo "FAIL: jobs reachable from pull_request must reject public fork heads:" >&2
  printf '  %s\n' $unguarded_fork_jobs >&2
  echo "      Require github.event.pull_request.head.repo.full_name == github.repository," >&2
  echo "      or explicitly exclude pull_request events from the job." >&2
  exit 1
fi

# 8. Secretless image build (ADR080 F2, closed by w2/m89). An image-build job
# executes repo-controlled code — Dockerfiles, go builds, yarn scripts — so any
# secret in its scope is exfiltratable by a poisoned dependency. Its whole
# credential budget is the registry-push GITHUB_TOKEN; deploy credentials live
# only in a separate environment-gated job consuming the build's digests. A
# secret referenced above `jobs:` leaks into every job, so a workflow that
# builds images must not declare one there either.
secretless_build_violations() {
  local wf
  for wf in $(collect_workflow_files); do
    awk -v file="$wf" '
      # Any secrets reference beyond the exact registry token is a violation,
      # including bracket/format() forms GitHub also resolves: strip the exact
      # allowed name to a marker, then flag every remaining secrets ref and any
      # longer name that merely starts with GITHUB_TOKEN.
      function has_disallowed_secret(text) {
        gsub(/secrets\.GITHUB_TOKEN/, "\001", text)
        return text ~ /\001[A-Za-z0-9_]/ || text ~ /secrets[.\[]/
      }
      function check_job() {
        if (job == "" || block !~ /docker\/build-push-action/) return
        if (header_secret || has_disallowed_secret(block)) {
          print file ":" job ": image-build job must reference no secret beyond the registry-push GITHUB_TOKEN"
        }
        if (block ~ /\n    environment:/) {
          print file ":" job ": image-build job must not carry an environment gate — deploy credentials belong to the deploy job"
        }
      }
      !in_jobs {
        if (has_disallowed_secret($0)) header_secret = 1
      }
      /^jobs:[[:space:]]*$/ { in_jobs = 1; next }
      # Fail closed when an image build appears before any recognized job
      # header — a reformatted/annotated job key must not exempt its job.
      in_jobs && job == "" && /docker\/build-push-action/ {
        print file ": docker/build-push-action outside a recognized job header — use the plain two-space `name:` job key form"
      }
      in_jobs && /^  [A-Za-z0-9_-]+:[[:space:]]*$/ {
        check_job()
        job = $1
        sub(/:$/, "", job)
        block = "\n" $0
        next
      }
      in_jobs && job != "" { block = block "\n" $0 }
      END { check_job() }
    ' "$wf"
  done
}

build_secret_violations="$(secretless_build_violations)"
if [ -n "$build_secret_violations" ]; then
  echo "FAIL: image builds must stay secretless (ADR080 F2, w2/m89):" >&2
  printf '%s\n' "$build_secret_violations" >&2
  exit 1
fi

# 9. Production-secret confinement. The table is shared with scripts/gh-secrets.sh
# so the environment that holds a secret and the job allowed to read it can
# never disagree. The canonical tree always uses the real table; a fixture run
# applies check 9 only when the self-test supplies PRODUCTION_SECRETS_FILE, so
# fixtures written for the other checks may name real secrets freely.
if [ "$canonical_tree" -eq 1 ]; then
  PRODUCTION_SECRETS_FILE="scripts/production-secrets.txt"
else
  PRODUCTION_SECRETS_FILE="${PRODUCTION_SECRETS_FILE:-/dev/null}"
fi
production_secret_violations() {
  local wf
  for wf in $(collect_workflow_files); do
    awk -v file="$wf" '
      FNR == NR {
        sub(/#.*/, "")
        if (NF >= 2) allowed[$1] = "," $2 ","
        next
      }
      function report(where, name, env) {
        if (env == "") {
          print file ":" where ": reads production secret " name " outside a production environment (allowed: " substr(allowed[name], 2, length(allowed[name]) - 2) ")"
        } else {
          print file ":" where ": reads production secret " name " in environment " env " (allowed: " substr(allowed[name], 2, length(allowed[name]) - 2) ")"
        }
      }
      function check_job(   text, name) {
        if (job == "") return
        text = block
        while (match(text, /secrets\.[A-Za-z0-9_]+/)) {
          name = substr(text, RSTART + 8, RLENGTH - 8)
          text = substr(text, RSTART + RLENGTH)
          if (!(name in allowed) || seen[job SUBSEP name]++) continue
          if (env == "" || index(allowed[name], "," env ",") == 0) report(job, name, env)
        }
      }
      /^jobs:[[:space:]]*$/ { in_jobs = 1; next }
      !in_jobs {
        text = $0
        while (match(text, /secrets\.[A-Za-z0-9_]+/)) {
          name = substr(text, RSTART + 8, RLENGTH - 8)
          text = substr(text, RSTART + RLENGTH)
          if (name in allowed) print file ":(workflow-level): references production secret " name " above jobs:, which exposes it to every job"
        }
        next
      }
      # A job header may carry a trailing comment; missing it would charge
      # the secrets of this job to the environment of the previous one.
      /^  [A-Za-z0-9_-]+:[[:space:]]*(#.*)?$/ {
        check_job()
        job = $1; sub(/:.*$/, "", job); block = ""; env = ""; env_block = 0
        next
      }
      /^    environment:/ {
        value = $0; sub(/^    environment:[[:space:]]*/, "", value); sub(/[[:space:]]*#.*/, "", value)
        if (value == "") { env_block = 1; next }
        # Flow mapping `{ name: x, url: … }` or a scalar.
        if (value ~ /^\{/) { if (match(value, /name:[[:space:]]*[^,}[:space:]]+/)) { value = substr(value, RSTART, RLENGTH); sub(/^name:[[:space:]]*/, "", value) } else value = "" }
        gsub(/["\047]/, "", value); env = value
      }
      env_block && /^      name:[[:space:]]*/ {
        env = $0; sub(/^      name:[[:space:]]*/, "", env); sub(/[[:space:]]*#.*/, "", env); gsub(/["\047]/, "", env)
      }
      env_block && !/^      / { env_block = 0 }
      { line = $0; sub(/^[[:space:]]*#.*/, "", line); block = block "\n" line }
      END { check_job() }
    ' "$PRODUCTION_SECRETS_FILE" "$wf"
  done
}

[ -r "$PRODUCTION_SECRETS_FILE" ] || { echo "FAIL: production-secret table missing: $PRODUCTION_SECRETS_FILE" >&2; exit 1; }
production_violations="$(production_secret_violations)"
if [ -n "$production_violations" ]; then
  echo "FAIL: production secrets must be read only inside their assigned environment (check 9, w3/m173, scripts/production-secrets.txt):" >&2
  printf '%s\n' "$production_violations" >&2
  exit 1
fi

# 10. Bounded CI jobs (w9/m173): every job holding no credential (no secret,
# environment or write token: exactly the class the retired `bex-ci` pool
# ran) sets a job-level timeout-minutes. Without one a hung step holds a
# runner for GitHub's 6 h default: a go-lint checkout stuck from 08:54 to
# 12:30 on 2026-11-06 left deploy tests queued, and hosted jobs still share
# the org's concurrent-job limit. Budgets are measured, about 2.5x the recent
# successful maximum. Credentialed production jobs are out of scope until
# their budgets are measured. A fixture run applies it only when the self-test
# sets BOUNDED_JOBS_CHECK=1, so fixtures written for other checks need no
# timeout.
unbounded_ci_jobs=""
if [ "$canonical_tree" -eq 1 ] || [ "${BOUNDED_JOBS_CHECK:-0}" = 1 ]; then
  unbounded_ci_jobs="$(python3 - "$WORKFLOWS_DIR" <<'PY'
import pathlib, re, sys
# Text scan (no YAML library assumed): jobs sit at two-space indent under
# `jobs:`, their keys at four.
# native-base-image.yml is byte-pinned by w8/m91's D3 custody transition
# (native-common-qualify.py D3_LEGACY_0941 "after"; test_d3_transition_pin_*):
# a timeout added there broke native-common (run of 759e447ff). Changing those
# bytes is w8's custody decision, as for check 6b; the 2026-11-09 re-pin
# carried only the hosted-runner move.
EXEMPT = {"native-base-image.yml"}
SECRET = re.compile(r"\$\{\{[^}]*\bsecrets\b|write-all")
HEADER_WRITE = re.compile(r"^  [A-Za-z0-9_-]+:\s*write\b")
JOB_CREDENTIAL = re.compile(r"^    environment:|^      [A-Za-z0-9_-]+:\s*write\b")
for path in sorted(pathlib.Path(sys.argv[1]).glob("*.yml")):
    if path.name in EXEMPT:
        continue
    job, in_jobs, header_credential = None, False, False
    def report():
        if job and runs and not bounded and not credential:
            print(f"{path.name}:{job}")
    for line in path.read_text().splitlines():
        if re.match(r"^jobs:\s*$", line):
            in_jobs = True
            continue
        if not in_jobs:
            if SECRET.search(line) or HEADER_WRITE.match(line):
                header_credential = True
            continue
        if re.match(r"^\S", line):
            report()
            job, in_jobs = None, False
            continue
        head = re.match(r"^  ([A-Za-z0-9_-]+):\s*$", line)
        if head:
            report()
            job, runs, bounded, credential = head.group(1), False, False, header_credential
            continue
        if job is None:
            continue
        if re.match(r"^    runs-on:", line):
            runs = True
        elif re.match(r"^    timeout-minutes:", line):
            bounded = True
        if SECRET.search(line) or JOB_CREDENTIAL.match(line):
            credential = True
    report()
PY
)"
fi
if [ -n "$unbounded_ci_jobs" ]; then
  echo "FAIL: uncredentialed jobs without a job-level timeout-minutes (a hung step would hold a runner for 6 h):" >&2
  echo "$unbounded_ci_jobs" >&2
  exit 1
fi

# The remaining checks pin the canonical tree's reviewed inventory + deploy.yml
# wiring; they don't apply to a fixture dir, so stop here under an override.
if [ "$canonical_tree" -eq 0 ]; then
  echo "PASS: third-party refs SHA-pinned, Node 20 absent, hosted-runner contract, credential boundary, public-fork isolation, production-secret confinement and bounded jobs (when requested) intact (fixture: $WORKFLOWS_DIR)"
  exit 0
fi

# 2. Reviewed inventory (SHA-pinned; sorted to match third_party_refs). Keep in
# lockstep with the workflow tree — a Dependabot actions bump updates both.
expected_actions='actions/cache@55cc8345863c7cc4c66a329aec7e433d2d1c52a9
actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1
actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e
actions/setup-node@820762786026740c76f36085b0efc47a31fe5020
actions/upload-artifact@cf430e030ddbb5b0abf93d22962f4752f3646cd9
anchore/sbom-action/download-syft@3ad7283483fc7af8ff2b4ea19663c2d5ca935e26
aquasecurity/setup-trivy@3fb12ec12f41e471780db15c232d5dd185dcb514
aquasecurity/trivy-action@ed142fd0673e97e23eac54620cfb913e5ce36c25
azure/setup-helm@9bc31f4ebc9c6b171d7bfbaa5d006ae7abdb4310
azure/setup-kubectl@829323503d1be3d00ca8346e5391ca0b07a9ab0d
docker/build-push-action@53b7df96c91f9c12dcc8a07bcb9ccacbed38856a
docker/login-action@dbcb813823bdd20940b903addbd779551569679f
docker/setup-buildx-action@37fe631027851001ddb9b187196cc803df7f5f0e
docker/setup-qemu-action@99012661954931238ded8c8b007157a8430204e1
oras-project/setup-oras@1d808f7d7f6995cc68b7bf507bfe5c5446e1dc9d
sigstore/cosign-installer@6f9f17788090df1f26f669e9d70d6ae9567deba6'

if ! diff -u <(printf '%s\n' "$expected_actions") <(third_party_refs); then
  echo "FAIL: GitHub Action refs differ from the reviewed SHA-pinned inventory" >&2
  exit 1
fi

if ! grep -Fq 'GITLEAKS_VERSION: 8.30.1' .github/workflows/deploy.yml \
  || ! grep -Fq 'GITLEAKS_LINUX_ARM64_SHA256: e4a487ee7ccd7d3a7f7ec08657610aa3606637dab924210b3aee62570fb4b080' .github/workflows/deploy.yml \
  || ! grep -Fq 'gitleaks_${GITLEAKS_VERSION}_linux_arm64.tar.gz' .github/workflows/deploy.yml \
  || ! grep -Fq 'gitleaks git --no-banner --redact --exit-code 1 --log-opts="$log_opts" .' .github/workflows/deploy.yml; then
  echo "FAIL: deploy must checksum-pin and execute the reviewed Gitleaks CLI scanner" >&2
  exit 1
fi
# Tuning the scan's config must never quietly turn the scan off: only the
# keys an allowlist needs to narrow named rules by shape are accepted (a list
# of allowed lines, so a new key, a path or commit skip, an empty or missing
# targetRules, or a line-wide match fails), and the default rules stay on.
if [ -e .gitleaks.toml ] && { ! grep -qx 'useDefault = true' .gitleaks.toml \
  || [ "$(grep -cx '\[\[allowlists\]\]' .gitleaks.toml)" -ne "$(grep -c '^targetRules = \["' .gitleaks.toml)" ] \
  || grep -Evq '^(#.*|\[extend\]|useDefault = true|\[\[allowlists\]\]|description = ".*"|targetRules = \[".*|regexTarget = "(secret|match)"|regexes = \[.*)?$' .gitleaks.toml; }; then
  echo "FAIL: .gitleaks.toml may only add allowlists that name their targetRules and match a secret or finding by shape, over the default rules" >&2
  exit 1
fi
if ! grep -Fq 'SHELLCHECK_VERSION=0.11.0' .github/workflows/scripts.yml \
  || ! grep -Fq 'SHELLCHECK_LINUX_ARM64_SHA256=12b331c1d2db6b9eb13cfca64306b1b157a86eb69db83023e261eaa7e7c14588' .github/workflows/scripts.yml \
  || ! grep -Fq 'shellcheck-v${SHELLCHECK_VERSION}.linux.aarch64.tar.xz' .github/workflows/scripts.yml; then
  echo "FAIL: scripts workflow must checksum-pin its rootless ShellCheck install" >&2
  exit 1
fi
if grep -Fq 'hashicorp/setup-terraform@' .github/workflows/infra.yml \
  || [ "$(grep -cF 'bash scripts/hashicorp-install.sh terraform "$TERRAFORM_VERSION" "$TERRAFORM_LINUX_ARM64_SHA256"' .github/workflows/infra.yml)" -ne 2 ] \
  || ! grep -Fq 'TERRAFORM_LINUX_ARM64_SHA256: 0ca5d6977c7c46bfa4bbe030030b911e897cf0cb72bff5525fb76c10f1c3409a' .github/workflows/infra.yml; then
  echo "FAIL: infra must install checksum-pinned Terraform without setup-terraform's host unzip dependency" >&2
  exit 1
fi
if grep -Fq 'hashicorp/setup-packer@' .github/workflows/snapshot.yml \
  || ! grep -Fq 'bash scripts/hashicorp-install.sh packer "$PACKER_VERSION" "$PACKER_LINUX_ARM64_SHA256"' .github/workflows/snapshot.yml \
  || ! grep -Fq 'PACKER_VERSION: 1.11.2' .github/workflows/snapshot.yml \
  || ! grep -Fq 'PACKER_LINUX_ARM64_SHA256: dd296d743dd4593304307583cff5290bba9b868fc2b0b605b64566f8141ca728' .github/workflows/snapshot.yml; then
  echo "FAIL: snapshot must install checksum-pinned Packer without setup-packer's host unzip dependency" >&2
  exit 1
fi

# w1/m59: a superseded deploy run must be neutralized at the write-back (never
# red, never a stale pin). block-eden-mono w1/m25: a push or scheduled run
# builds only when the `changes` gate found a deployable commit after a
# production pin at least 6 hours old.
# Pin both so neither can silently drift.
if ! grep -Eq '^  changes:' .github/workflows/deploy.yml \
  || ! grep -Fq 'python3 scripts/deploy_latency.py --changed --min-pin-age-hours "$min_age"' .github/workflows/deploy.yml \
  || ! grep -Fq 'push | schedule) min_age=6 ;;' .github/workflows/deploy.yml \
  || ! grep -Fq "if: needs.changes.outputs.deploy == 'true' && github.ref == 'refs/heads/main'" .github/workflows/deploy.yml \
  || [ "$(grep -cF 'bash scripts/deploy-superseded.sh "$GITHUB_SHA"' .github/workflows/deploy.yml)" -lt 1 ]; then
  echo "FAIL: deploy.yml lost the w1/m25 scheduled change gate or the w1/m59 write-back guard (scripts/deploy-superseded.sh)" >&2
  exit 1
fi
# actions/checkout points origin/main at the run's own commit; the gate must
# read main's live tip, or a run queued behind a deploy deploys a second time.
if ! grep -Fq 'run: git fetch --no-tags --filter=blob:none origin +refs/heads/main:refs/remotes/origin/main' .github/workflows/deploy.yml; then
  echo "FAIL: deploy.yml's changes gate must fetch main's live tip before deploy_latency.py reads the pin" >&2
  exit 1
fi
if [ "$(grep -cF "env.SUPERSEDED != 'true'" .github/workflows/deploy.yml)" -lt 4 ]; then
  echo "FAIL: deploy.yml rollout steps must gate on env.SUPERSEDED so a mid-build supersession concludes neutral (w1/m59)" >&2
  exit 1
fi
if [ "$(grep -cF "steps.app_kubeconfig.outcome == 'success'" .github/workflows/deploy.yml)" -lt 4 ]; then
  echo "FAIL: deploy.yml always-run rollout checks must require a successfully fetched app kubeconfig" >&2
  exit 1
fi

# Anti-vacuity for check 4: the pin-coverage loop passes trivially over a tree
# with no admin.conf fetchers, so pin the known three (deploy, app-cluster,
# openbao-restore-drill). Removing a fetcher is fine; silently having none is not.
if [ "$(admin_conf_fetchers | wc -l | tr -d ' ')" -lt 3 ]; then
  echo "FAIL: expected at least 3 workflows fetching admin.conf over SSH — the pin-coverage check (4) has gone vacuous" >&2
  exit 1
fi

if [ "${#restore_workflow_files[@]}" -lt 1 ]; then
  echo "FAIL: expected at least 1 workflow running scripts/restore-*.sh — the age-key check (4b) has gone vacuous" >&2
  exit 1
fi

# Anti-vacuity for check 5: it inspects only the lines it recognizes as
# runs-on, so a reformatted tree with none would pass. Pin a floor of
# approved labels.
if [ "$(grep -RhE '^    runs-on: (ubuntu-24\.04-arm|ubuntu-latest)[[:space:]]*$' .github/workflows lego/operator/.github/workflows 2>/dev/null | wc -l | tr -d ' ')" -lt 20 ]; then
  echo "FAIL: expected GitHub-hosted runs-on across the workflow tree — the runner check (5) has gone vacuous" >&2
  exit 1
fi

echo "PASS: third-party actions SHA-pinned, reviewed inventory current, Node 20 absent, Gitleaks + supersession wiring intact, admin.conf fetchers host-key pinned with valid kubeconfig lifetimes, GitHub-hosted runner contract, bounded uncredentialed jobs, credential boundary, public-fork isolation and production-secret confinement intact"
