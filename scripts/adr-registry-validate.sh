#!/usr/bin/env bash
# ADR registry integrity (w6/m40). ADR numbers are bex's citation system: the
# security-review lineage alone is a fourteen-document chain navigated by
# number, and CLAUDE.md / DO_NOT_DO.md / every milestone README point at
# architectural decisions the same way.
#
# Three collisions had accumulated before this guard (ADR040, ADR049, ADR060),
# and each was CREATED by a well-intentioned rename meant to fix an earlier
# one — the round-7 security review records "Renamed from ADR058 to resolve the
# number collision with ADR058-release-engineering", and it collided again.
# The renumbering was a one-off; this guard is what stops the fourth.
#
# Checks (all fail closed):
#   1. no two docs/ADR*.md files claim the same number
#   2. every docs/ADR*.md appears in the docs catalog (docs/CLAUDE.md)
#   3. no ADRnnn-<slug>.md reference anywhere resolves to a missing file
#
# ADR_DIR, ADR_INDEX, and SCAN_ROOT are overridable so the self-test
# (scripts/adr-registry-validate.test.sh) can run every check against throwaway
# fixture trees instead of the real repository.
set -euo pipefail
cd "$(dirname "$0")/.."

ADR_DIR="${ADR_DIR:-docs}"
# The catalog moved out of the root CLAUDE.md into the cascading docs/CLAUDE.md
# when the agent docs were compacted; the root file now carries only key entry
# points, so checking it would fail for every ADR outside that short list.
ADR_INDEX="${ADR_INDEX:-docs/CLAUDE.md}"
SCAN_ROOT="${SCAN_ROOT:-.}"
fail=0

shopt -s nullglob
adrs=("$ADR_DIR"/ADR*.md)
shopt -u nullglob

if [ "${#adrs[@]}" -eq 0 ]; then
  echo "FAIL: no ADR files found under $ADR_DIR" >&2
  exit 1
fi

# --- 1. duplicate numbers -------------------------------------------------
# Two documents claiming one number make every bare "ADRnnn" citation
# ambiguous, which is the defect this guard exists for.
declare -A claimed_by=()
for path in "${adrs[@]}"; do
  base="${path##*/}"
  if [[ ! "$base" =~ ^ADR([0-9]{3}) ]]; then
    echo "FAIL: $base does not match the ADR<nnn>-<slug>.md naming convention" >&2
    fail=1
    continue
  fi
  num="${BASH_REMATCH[1]}"
  if [ -n "${claimed_by[$num]:-}" ]; then
    echo "FAIL: ADR number $num is claimed by two documents:" >&2
    echo "        ${claimed_by[$num]}" >&2
    echo "        $base" >&2
    echo "      Renumber one (keep the number on the more-cited document) and" >&2
    echo "      record the old->new mapping in its header." >&2
    fail=1
  else
    claimed_by[$num]="$base"
  fi
done

# --- 2. every ADR is indexed ---------------------------------------------
# An unindexed ADR is invisible: the docs catalog is what an agent reads to
# discover decisions, so one absent from it effectively does not exist.
if [ ! -f "$ADR_INDEX" ]; then
  echo "FAIL: index file not found: $ADR_INDEX" >&2
  exit 1
fi
for path in "${adrs[@]}"; do
  base="${path##*/}"
  if ! grep -qF "$base" "$ADR_INDEX"; then
    echo "FAIL: $base is not referenced in $ADR_INDEX's docs index" >&2
    echo "      Add a one-line entry describing what it decides." >&2
    fail=1
  fi
done

# --- 3. no dangling ADR reference ----------------------------------------
# A reference to a renamed or deleted ADR is exactly the rot a renumbering
# leaves behind, so the guard that permits renumbering must also catch it.
#
# Bare "ADRnnn" rename notes in the ADR headers can't match — the pattern
# requires the full -<slug>.md — so a prose citation of a superseded number is
# inert by construction and only real file references are checked.
#
# The slug charset excludes '.' deliberately: no real ADR slug contains one,
# and a looser class swallows prose ellipses like `docs/ADR047-...md` (a real
# hit the first run produced) and reports them as broken references.
scan_filters=(--include='*.md' --include='*.go' --include='*.sh'
  --include='*.yaml' --include='*.yml' --include='*.ts' --include='*.tsx'
  --exclude-dir=node_modules --exclude-dir=.git --exclude-dir=vendor
  --exclude-dir=dist --exclude-dir=bin
  # The self-test builds fixture registries whose ADR names deliberately do not
  # exist; scanning it would report its own fixtures as dangling references.
  --exclude=adr-registry-validate.test.sh)

refs="$(grep -rhoE "${scan_filters[@]}" 'ADR[0-9]{3}-[A-Za-z0-9-]+\.md' "$SCAN_ROOT" 2>/dev/null |
  LC_ALL=C sort -u || true)"

for ref in $refs; do
  if [ ! -f "$ADR_DIR/$ref" ]; then
    echo "FAIL: dangling ADR reference: $ref (no such file in $ADR_DIR/)" >&2
    grep -rnF "${scan_filters[@]}" "$ref" "$SCAN_ROOT" 2>/dev/null | head -5 |
      sed 's/^/        /' >&2 || true
    fail=1
  fi
done

[ "$fail" -eq 0 ] || exit 1

echo "PASS: ADR registry — ${#adrs[@]} ADRs, no duplicate numbers, all indexed, no dangling references"
