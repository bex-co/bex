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
#   - Markdown soft limit (w10/m607): a *.md file of 768 KiB or more is
#     refused, and one of 512 KiB or more draws a warning, so a growing
#     document is split long before the 1 MiB refusal. The parity ledger,
#     amended 40-50 KB a day in November 2026 by every workstream, reached
#     1 MiB once and turned main red (w10/053's d2d63d5cda); the warning band
#     gives about five days of that growth to split, and the refusal leaves
#     256 KiB for the push that splits. The size counted is the file at HEAD.
#
#   bash scripts/repo-artifact-guard.sh            # every blob in HEAD's tree
#   bash scripts/repo-artifact-guard.sh <base>     # every blob added in base..HEAD,
#                                                  # and every *.md it adds or changes
#
# The range form also catches a big file that a later commit deletes, and a
# stale checkout replaying pre-rewrite history through `git pull --rebase`.
# In a partial clone made with `--filter=blob:limit=1m`, a missing blob is by
# definition over the limit; rev-list --missing=print reports it without
# fetching it.
set -euo pipefail

limit=$((1024 * 1024))
md_warn=$((512 * 1024))
md_refuse=$((768 * 1024))
warn_kib="$((md_warn / 1024)) KiB" refuse_kib="$((md_refuse / 1024)) KiB"
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
# Markdown sizes at HEAD: every tracked *.md, or in the range form those a
# pushed commit adds or changes. A blob missing from a size-filtered clone is
# over 1 MiB, which the check above already names; GIT_NO_LAZY_FETCH keeps it
# unfetched.
md_blobs=$(git -c core.quotePath=false ls-tree -r HEAD |
  awk -F'\t' '$2 ~ /\.md$/ { split($1, f, " "); if (f[2] == "blob") print f[3] " " $2 }')
if [ $# -gt 0 ]; then
  md_blobs=$(awk 'NR == FNR { changed[$0] = 1; next } { path = $0; sub(/^[^ ]+ /, "", path) } path in changed' \
    <(git -c core.quotePath=false log --format= --name-only --diff-filter=ACMR --no-renames "$1..HEAD" -- '*.md') - <<<"$md_blobs")
fi
md_sizes=$(GIT_NO_LAZY_FETCH=1 git cat-file --batch-check='%(objectsize) %(rest)' <<<"$md_blobs" 2>/dev/null | grep -v ' missing$' || true)
md_over=$(awk -v refuse="$md_refuse" -v kib="$refuse_kib" '$1 >= refuse { size = $1; sub(/^[^ ]+ /, ""); print $0 " (" size " bytes >= " kib ")" }' <<<"$md_sizes")
while IFS= read -r warned; do
  [ -n "$warned" ] || continue
  path=${warned#* }
  echo "repo-artifact-guard: warning: $path is ${warned%% *} bytes, past the $warn_kib markdown warning band; split it before it reaches $refuse_kib, when this guard refuses it" >&2
  [ "${GITHUB_ACTIONS:-}" != true ] || echo "::warning file=$path,title=Markdown file near the size limit::$path is ${warned%% *} bytes; split it before it reaches $refuse_kib"
done < <(awk -v warn="$md_warn" -v refuse="$md_refuse" '$1 >= warn && $1 < refuse' <<<"$md_sizes")

if [ -n "$bad" ]; then
  echo "repo-artifact-guard: build artifacts or evidence dumps must not be committed:" >&2
  printf '  %s\n' "${bad//$'\n'/$'\n  '}" >&2
  echo "repo-artifact-guard: upload them to object storage and record the sha256 + link in the board markdown" >&2
fi
if [ -n "$md_over" ]; then
  echo "repo-artifact-guard: markdown files must stay under $refuse_kib (the soft limit before the 1 MiB blob refusal):" >&2
  printf '  %s\n' "${md_over//$'\n'/$'\n  '}" >&2
  echo "repo-artifact-guard: split each into files of its own, leaving a pointer where each part was, as docs/ADR018/ split the parity ledger (w10/m607); a board README moves finished history out instead" >&2
fi
[ -z "$bad" ] && [ -z "$md_over" ]
