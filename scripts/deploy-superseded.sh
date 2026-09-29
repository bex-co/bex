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
# digest write-back is NOT a supersession. Match deploy.yml's push paths,
# including its CLI exclusion: CLI-only commits schedule no replacement run.
#
# Newer commits are split in two. MANIFESTS — what Argo CD syncs from main
# (deploy/gitops, deploy/opensandbox, lego/operator/config) plus the workflow —
# must never receive images built from an older tree: that is a supersession.
# IMAGE SOURCE — everything else under lego/ and dashboard/ — only means newer
# images are coming. Pinning this run's images onto unchanged manifests is an
# ordinary deploy of this commit, and runs are serialized, so the newer run pins
# right after. Treating source drift as supersession starved production: with
# code landing more often than one ~50-minute run, no run could ever pin (no pin
# from cae30d1e0, 2026-09-28 16:41Z, until this change; w1/116).
#
# Usage:   scripts/deploy-superseded.sh <git-sha>
# Exit:    0 = superseded   (newer manifests on origin/main: never pin these images)
#          1 = current      (no production-input drift beyond generated digests)
#          2 = error        (could not determine — callers treat as "not skipped":
#                            the pre-build check proceeds and the write-back guard,
#                            the real correctness gate, re-checks before pinning)
#          3 = newer source (only image source moved: a newer run is queued, so the
#                            pre-build check may skip, but a finished build pins)
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
  deploy/opensandbox deploy/gitops .github/workflows/deploy.yml lego/operator/config \
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
    echo "superseded by newer production manifests on origin/main ($(git rev-parse --short origin/main)); this run's images will not be built/pinned"
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
        r"(?m)^(\s+tag: [^@\s]+@sha256:)[0-9a-f]{64}$",
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
  0) ;; # no manifest drift beyond the five generated digest fields
  1)
    echo "superseded by a substantive generated-file change on origin/main ($(git rev-parse --short origin/main)); this run's images will not be built/pinned"
    exit 0
    ;;
  *)
    exit 2
    ;;
esac

# Manifests are current. Newer image source alone means a newer run is queued.
git diff --quiet "$SHA" origin/main -- lego dashboard \
  ':(exclude)lego/cli/**' ':(exclude)lego/operator/config/**'
case $? in
  0) exit 1 ;;
  1)
    echo "newer image source on origin/main ($(git rev-parse --short origin/main)) with unchanged manifests: a newer run is queued, and these images are safe to pin"
    exit 3
    ;;
  *)
    echo "deploy-superseded: git diff failed" >&2
    exit 2
    ;;
esac
