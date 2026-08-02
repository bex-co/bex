#!/usr/bin/env bash
# Detect whether THIS deploy run has been superseded — i.e. whether a newer
# deploy-triggering commit has landed on origin/main since <git-sha> (w1/m59).
# The single source of truth shared by two consumers in
# .github/workflows/deploy.yml:
#   - the pre-build `check-supersession` job (skip a queued-behind-newer run
#     before it wastes an image build), and
#   - the write-back guard (never pin images built from stale production inputs).
#
# "Deploy-triggering" = any change under the production-input path filter,
# EXCLUDING the four generated image-digest fields — a preceding run's [skip ci]
# digest write-back is NOT a supersession. This filter must match the one the
# write-back guard has always used.
#
# Usage:   scripts/deploy-superseded.sh <git-sha>
# Exit:    0 = superseded (a newer deploy-triggering commit is on origin/main)
#          1 = current    (no production-input drift beyond generated digests)
#          2 = error       (could not determine — callers treat as "not skipped":
#                           the pre-build check proceeds and the write-back guard,
#                           the real correctness gate, re-checks before pinning)
set -uo pipefail

SHA="${1:?usage: deploy-superseded.sh <git-sha>}"

# Bring origin/main's tip into view. A shallow checkout still has <git-sha> (the
# run's HEAD); fetching main's tip is enough for a tree-level diff.
if ! git fetch --no-tags origin main >/dev/null 2>&1; then
  echo "deploy-superseded: could not fetch origin/main" >&2
  exit 2
fi

# The production-input path filter, minus the four generated digest files (a
# preceding run's [skip ci] pin must not read as a supersession). Kept identical
# to the write-back guard.
git diff --quiet "$SHA" origin/main -- \
  lego dashboard deploy/opensandbox deploy/gitops .github/workflows/deploy.yml \
  ':(exclude)deploy/gitops/base/bex.yaml' \
  ':(exclude)deploy/gitops/base/dashboard.yaml' \
  ':(exclude)deploy/opensandbox/kustomization.yaml' \
  ':(exclude)deploy/gitops/base/values/opensandbox-controller.values.yaml'
case "$?" in
  0)
    exit 1 # no drift beyond generated digests → current
    ;;
  1)
    echo "superseded by newer production inputs on origin/main ($(git rev-parse --short origin/main)); this run's images will not be built/pinned"
    exit 0
    ;;
  *)
    echo "deploy-superseded: git diff failed" >&2
    exit 2
    ;;
esac
