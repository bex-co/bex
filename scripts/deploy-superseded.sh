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
# Newer MANIFESTS are not a reason to skip either (w4/439). Argo CD auto-syncs
# them from main the moment they land, so they already run on the PREVIOUS
# pin's images, which are older still. Refusing to pin this run's images never
# kept older images off newer manifests; it only kept production on the oldest
# ones. With manifest commits landing every 20-100 minutes against 1.5-2.6 h
# runs, every finished run was refused and production went 11 h without a pin
# (2026-10-29/30). Pinning this run's images narrows the gap, and the queued
# run closes it. Two cases still skip: a generated digest field reshaped
# beyond its digest (the write-back's field rewrite would target the wrong
# shape), and a commit that is not newer than the current pin (a re-run of an
# old run must never roll production back).
#
# Usage:   scripts/deploy-superseded.sh <git-sha>
# Exit:    0 = superseded   (never pin these images: a generated digest field
#                            was reshaped, or the current pin is newer)
#          1 = current      (no production-input drift beyond generated digests)
#          2 = error        (could not determine — callers treat as "not skipped":
#                            the pre-build check proceeds and the write-back guard,
#                            the real correctness gate, re-checks before pinning)
#          3 = newer source (only image source moved: a newer run is queued, but
#                            both the pre-build check and the write-back guard
#                            build and pin this commit — skipping here starved
#                            production under steady pushes)
#          4 = newer manifests (already live through Argo on the older pinned
#                            images: the pre-build check skips, since the
#                            queued run builds the newer tree, but a run that
#                            already built pins its images)
set -uo pipefail

SHA="${1:?usage: deploy-superseded.sh <git-sha>}"

# Bring origin/main's tip into view. A shallow checkout still has <git-sha> (the
# run's HEAD); fetching main's tip is enough for a tree-level diff.
if ! git fetch --no-tags origin main >/dev/null 2>&1; then
  echo "deploy-superseded: could not fetch origin/main" >&2
  exit 2
fi

MANIFEST_PATHS=(deploy/opensandbox deploy/gitops .github/workflows/deploy.yml lego/operator/config)

# Name the commits that moved the manifests, not origin/main's tip: the tip is
# often an unrelated docs commit, which once sent a 2-hour deploy delay's
# diagnosis after the wrong commit (w2/050). The fetch above leaves the range
# shallow on CI, so fall back to the tip when the log cannot be read.
superseders() {
  local commits
  if [ "$(git rev-parse --is-shallow-repository 2>/dev/null)" = true ]; then
    # Only on the superseded path, so the extra fetch is rare and bounded.
    git fetch --no-tags --deepen=500 origin main >/dev/null 2>&1 || true
  fi
  if git merge-base --is-ancestor "$SHA" origin/main 2>/dev/null; then
    commits="$(git log --format=%h "$SHA..origin/main" -- "$@" 2>/dev/null | tr '\n' ' ' | sed 's/ $//')"
  fi
  printf '%s' "${commits:-unknown, history too shallow; origin/main is $(git rev-parse --short origin/main)}"
}

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
manifests_moved=false
case "$diff_rc" in
  0)
    ;;
  1)
    manifests_moved=true
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


def generated_field(revision, path, pattern):
    """Return (prefix, rest): the field's captured prefix and the file with
    only the digest replaced."""
    try:
        text = subprocess.check_output(
            ["git", "show", f"{revision}:{path}"], text=True, stderr=subprocess.DEVNULL
        )
    except subprocess.CalledProcessError as error:
        raise RuntimeError(f"could not read {path} at {revision}") from error
    matches = list(re.finditer(pattern, text))
    if len(matches) != 1:
        return None, text  # the field is gone or duplicated: reshaped
    return matches[0].group(1), re.sub(pattern, r"\g<1><generated>", text)


# Exit codes: 0 = only digests differ; 1 = a generated field was reshaped (the
# image name or tag shape changed, so a write-back would target the wrong
# shape); 5 = the rest of a generated file differs, which is an ordinary
# manifest change (w4/465).
reshaped = False
moved = False
try:
    for path, pattern in generated.items():
        old_prefix, old_rest = generated_field(revisions[0], path, pattern)
        new_prefix, new_rest = generated_field(revisions[1], path, pattern)
        if old_prefix is None or new_prefix is None or old_prefix != new_prefix:
            reshaped = True
        elif old_rest != new_rest:
            moved = True
except RuntimeError as error:
    print(f"deploy-superseded: {error}", file=sys.stderr)
    raise SystemExit(2)

raise SystemExit(1 if reshaped else 5 if moved else 0)
PY
generated_rc=$?
case "$generated_rc" in
  0) ;; # no manifest drift beyond the five generated digest fields
  5) manifests_moved=true ;; # an ordinary edit elsewhere in a generated file: same as any manifest path
  1)
    echo "superseded by a generated-field reshape on origin/main (changed by: $(superseders "${MANIFEST_PATHS[@]}")); this run's images will not be built/pinned"
    exit 0
    ;;
  *)
    exit 2
    ;;
esac

# Every pin path refuses a commit older than the current pin. Runs are
# serialized in push order, so only a re-run of an old run trips this. The pin
# commit names its producer; without one in reach (a fresh repository, history
# beyond the deepen) there is nothing to roll back over.
pinned_producer() {
  # CI checks out one commit: reach back far enough for the last pin and its
  # producer, as superseders() does.
  if [ "$(git rev-parse --is-shallow-repository 2>/dev/null)" = true ]; then
    git fetch --no-tags --deepen=500 origin main >/dev/null 2>&1 || true
  fi
  git log origin/main -1 --format=%s -E --grep='^chore\(deploy\): pin platform images to [0-9a-f]{12}' 2>/dev/null |
    sed -En 's/^chore\(deploy\): pin platform images to ([0-9a-f]{12}).*/\1/p'
}
producer="$(pinned_producer)"
if [ -n "$producer" ] && git rev-parse -q --verify "$producer^{commit}" >/dev/null 2>&1 &&
  ! git merge-base --is-ancestor "$producer" "$SHA" 2>/dev/null; then
  echo "the current pin ($producer) is not an ancestor of this commit; pinning it would roll production back"
  exit 0
fi

if [ "$manifests_moved" = true ]; then
  echo "newer production manifests on origin/main (changed by: $(superseders "${MANIFEST_PATHS[@]}")) already run on the older pinned images; these images narrow that gap and are safe to pin"
  exit 4
fi

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
