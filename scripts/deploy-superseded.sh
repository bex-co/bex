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
# EXCLUDING the five generated image-digest fields — a preceding run's [skip ci]
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

# First compare every production input except the five files that contain one
# generated digest each. The second pass below compares those files after
# replacing only their generated field, so a real manifest change is never
# hidden by a whole-file exclusion.
git diff --quiet "$SHA" origin/main -- \
  lego dashboard deploy/opensandbox deploy/gitops .github/workflows/deploy.yml \
  ':(exclude)deploy/gitops/base/bex.yaml' \
  ':(exclude)deploy/gitops/base/dashboard.yaml' \
  ':(exclude)deploy/opensandbox/kustomization.yaml' \
  ':(exclude)deploy/gitops/base/values/opensandbox-controller.values.yaml' \
  ':(exclude)lego/operator/config/api/deployment.yaml'
diff_rc=$?
case "$diff_rc" in
  0)
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

python3 - "$SHA" origin/main <<'PY'
import re
import subprocess
import sys

revisions = sys.argv[1:]
generated = {
    "deploy/gitops/base/bex.yaml":
        r"(?m)^(\s+- controller=[^@\n]+@sha256:)[0-9a-f]{64}$",
    "deploy/gitops/base/dashboard.yaml":
        r"(?m)^(\s+- dashboard=[^@\n]+@sha256:)[0-9a-f]{64}$",
    "deploy/opensandbox/kustomization.yaml":
        r"(?m)(^  - name: opensandbox-server\n    newName: [^\n]+\n    digest: sha256:)[0-9a-f]{64}$",
    "deploy/gitops/base/values/opensandbox-controller.values.yaml":
        r"(?m)^(\s+tag: v0\.2\.0-bex-snapjobns@sha256:)[0-9a-f]{64}$",
    "lego/operator/config/api/deployment.yaml":
        r"(?m)^(\s+value: ghcr.io/bex-co/bex-agent-sandbox)(?::[^@\s]+|@sha256:[0-9a-f]{64})$",
}


def normalized(revision, path, pattern):
    try:
        text = subprocess.check_output(
            ["git", "show", f"{revision}:{path}"], text=True, stderr=subprocess.DEVNULL
        )
    except subprocess.CalledProcessError as error:
        raise RuntimeError(f"could not read {path} at {revision}") from error
    rendered, count = re.subn(pattern, r"\g<1><generated>", text)
    if count != 1:
        raise RuntimeError(f"expected exactly one generated digest field in {path}")
    return rendered


try:
    changed = any(
        normalized(revisions[0], path, pattern)
        != normalized(revisions[1], path, pattern)
        for path, pattern in generated.items()
    )
except RuntimeError as error:
    print(f"deploy-superseded: {error}", file=sys.stderr)
    raise SystemExit(2)

raise SystemExit(1 if changed else 0)
PY
generated_rc=$?
case "$generated_rc" in
  0)
    exit 1 # no drift beyond the five generated digest fields → current
    ;;
  1)
    echo "superseded by a substantive generated-file change on origin/main ($(git rev-parse --short origin/main)); this run's images will not be built/pinned"
    exit 0
    ;;
  *)
    exit 2
    ;;
esac
