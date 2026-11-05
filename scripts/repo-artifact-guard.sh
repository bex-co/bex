#!/usr/bin/env bash
# repo-artifact-guard.sh — keep build artifacts and evidence dumps out of git
# (user decision 2026-11-04, w6/100 D10). The 2026-11-04 history rewrite cut
# the repository from ~2 GB of .pm evidence (wheels, tarballs, SPDX copies, a
# compiled activator binary) back to ~100 MB; this guard keeps it there.
#
#   - No blob anywhere may reach 1 MiB (git's blob:limit=1m omits blobs of
#     exactly 1 MiB too, so the local check uses the same bound).
#   - Nothing under .pm/ may carry a binary/archive extension. The board is
#     markdown and small text; artifacts go to object storage, and the board
#     records their sha256 and link.
#   - No evidence dumps, whatever their size: no .pm directory named
#     *evidence*, no .pm json/jsonl/log/stdout/stderr or *evidence*.txt, no
#     mobile/e2e/artifacts/, no image at the repo root.
#
#   bash scripts/repo-artifact-guard.sh            # every blob in HEAD's tree
#   bash scripts/repo-artifact-guard.sh <base>     # every blob added in base..HEAD
#
# The range form also catches a big file that a later commit deletes, and a
# stale checkout replaying pre-rewrite history through `git pull --rebase`.
# In a partial clone made with `--filter=blob:limit=1m`, a missing blob is by
# definition over the limit; rev-list --missing=print reports it without
# fetching it.
set -euo pipefail

limit=$((1024 * 1024))
pm_ext='whl|gz|tgz|tar|zip|bin|xz|zst|bz2|7z|jar|so|dylib|exe|png|jpg|jpeg|gif|webp|pdf|mp4|mov|sqlite|db'
dump_re='^\\.pm/(.*/)?[^/]*evidence[^/]*/|^\\.pm/.*\\.(json|jsonl|log|stdout|stderr)$|^\\.pm/.*(evidence[^/]*|\\.log)\\.txt$|^mobile/e2e/artifacts/|^[^/]+\\.(png|jpe?g|gif|webp)$'
if [ $# -gt 0 ]; then
  revs=("$1..HEAD")
  # Every path a pushed commit adds, including ones a later commit deletes.
  paths=$(git -c core.quotePath=false log --format= --name-only --diff-filter=ACR --no-renames "$1..HEAD")
else
  revs=(--no-walk HEAD)
  paths=$(git -c core.quotePath=false ls-tree -r --name-only HEAD)
fi

# Sizes per object: rev-list lists each blob once, under the first path that
# reaches it, so names come from $paths, not from here.
objects=$(git rev-list --objects --missing=print "${revs[@]}")
bad=$(
  {
    # Missing (promisor-filtered) objects: only blobs are ever filtered out.
    grep '^?' <<<"$objects" | sed 's/^?//' |
      awk '{print $1 " (not fetched: 1 MiB or more in a size-filtered clone; name it with git log --all --find-object=" $1 ")"}' || true
    { grep -v '^?' <<<"$objects" || true; } |
      git cat-file --batch-check='%(objectname) %(objecttype) %(objectsize) %(rest)' |
      awk -v limit="$limit" '$2 == "blob" && $3 >= limit {
        path = $0; sub(/^[^ ]+ [^ ]+ [^ ]+ ?/, "", path)
        print path " (" $3 " bytes >= 1 MiB)"
      }'
    awk -v ext="$pm_ext" -v dump="$dump_re" '{ p = tolower($0) }
      p ~ ("^\\.pm/.*\\.(" ext ")$") { print $0 " (binary/archive under .pm)" }
      p ~ dump { print $0 " (evidence dump)" }' <<<"$paths"
  } | sort -u
)
if [ -n "$bad" ]; then
  echo "repo-artifact-guard: build artifacts or evidence dumps must not be committed:" >&2
  printf '  %s\n' "${bad//$'\n'/$'\n  '}" >&2
  echo "repo-artifact-guard: upload them to object storage and record the sha256 + link in the board markdown" >&2
  exit 1
fi
