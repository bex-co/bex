#!/usr/bin/env bash
# Self-test for scripts/adr-registry-validate.sh (w6/m40 t006). The
# anti-tautology rule: a guard with no proven red case proves nothing — and
# this guard's whole value is that it fails, since the collisions it prevents
# were each introduced by a rename nobody noticed.
#
# Each of the three checks gets its own isolated red case, so a single bug
# cannot make them all pass together, plus an assertion that the failure
# message names the offender (a guard whose output does not say what to fix
# gets disabled). Finally the real tree must pass end to end.
set -euo pipefail

here="$(cd "$(dirname "$0")" && pwd)"
SCRIPT="$here/adr-registry-validate.sh"
[ -x "$SCRIPT" ] || {
  echo "FAIL: $SCRIPT not executable" >&2
  exit 1
}

fails=0
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

# fixture <dir> — a minimal valid registry: two ADRs, both indexed.
fixture() {
  local dir="$1"
  mkdir -p "$dir/docs"
  printf '# ADR001 — first\n' >"$dir/docs/ADR001-first.md"
  printf '# ADR002 — second\n' >"$dir/docs/ADR002-second.md"
  printf '# index\n- [docs/ADR001-first.md](docs/ADR001-first.md) — first\n- [docs/ADR002-second.md](docs/ADR002-second.md) — second\n' >"$dir/INDEX.md"
}

# run <dir> — the validator scoped to one fixture tree, stderr only. Paths are
# absolute because the validator cd's to its own repo root.
run() { { ADR_DIR="$1/docs" ADR_INDEX="$1/INDEX.md" SCAN_ROOT="$1" "$SCRIPT" >/dev/null; } 2>&1; }

# assert <label> <want-rc> <dir> [expected-stderr-substring]
assert() {
  local label="$1" want="$2" dir="$3" needle="${4:-}"
  set +e
  local err
  err="$(run "$dir")"
  local got=$?
  set -e
  if [ "$got" -ne "$want" ]; then
    echo "FAIL: $label — exit $got, want $want" >&2
    printf '%s\n' "$err" | sed 's/^/        /' >&2
    fails=$((fails + 1))
    return
  fi
  if [ -n "$needle" ] && ! printf '%s' "$err" | grep -qF "$needle"; then
    echo "FAIL: $label — stderr did not name '$needle'" >&2
    printf '%s\n' "$err" | sed 's/^/        /' >&2
    fails=$((fails + 1))
    return
  fi
  echo "ok: $label (exit $got)"
}

# --- green baseline: a clean fixture must pass ----------------------------
green="$tmp/green"
fixture "$green"
assert "clean registry passes" 0 "$green"

# --- red 1: duplicate ADR number ------------------------------------------
dup="$tmp/dup"
fixture "$dup"
printf '# ADR002 — collides\n' >"$dup/docs/ADR002-collides.md"
printf -- '- [docs/ADR002-collides.md](docs/ADR002-collides.md) — collides\n' >>"$dup/INDEX.md"
assert "duplicate number is red" 1 "$dup" "ADR number 002 is claimed by two documents"

# --- red 2: ADR missing from the index ------------------------------------
unindexed="$tmp/unindexed"
fixture "$unindexed"
printf '# ADR003 — orphan\n' >"$unindexed/docs/ADR003-orphan.md"
assert "unindexed ADR is red" 1 "$unindexed" "ADR003-orphan.md is not referenced"

# --- red 3: dangling reference to a missing ADR ---------------------------
dangling="$tmp/dangling"
fixture "$dangling"
printf 'see [ADR404](ADR404-missing.md)\n' >>"$dangling/docs/ADR001-first.md"
assert "dangling reference is red" 1 "$dangling" "dangling ADR reference: ADR404-missing.md"

# --- each red case must fail for its OWN reason ---------------------------
# The needle assertions prove each message is specific; this additionally pins
# that the duplicate fixture emits exactly one FAIL class, so a bug in one
# check cannot make the others look like they fired.
dupclasses="$(run "$dup" | grep -c '^FAIL: ' || true)"
if [ "$dupclasses" -ne 1 ]; then
  echo "FAIL: duplicate fixture reported $dupclasses FAIL lines, want exactly 1 (checks are leaking into each other)" >&2
  fails=$((fails + 1))
else
  echo "ok: duplicate fixture fails for its own reason only"
fi

# --- prose ellipses must not be mistaken for references -------------------
# `docs/ADR047-...md` appears in a .pm archive as shorthand; an over-loose
# slug charset reported it as a broken reference on the guard's first run.
ellipsis="$tmp/ellipsis"
fixture "$ellipsis"
printf 'the design in docs/ADR047-...md was already accepted\n' >>"$ellipsis/docs/ADR001-first.md"
assert "prose ellipsis is not a reference" 0 "$ellipsis"

# --- the real tree must pass ----------------------------------------------
if "$SCRIPT" >/dev/null 2>&1; then
  echo "ok: real repository tree passes"
else
  echo "FAIL: real repository tree does not pass the ADR registry guard" >&2
  fails=$((fails + 1))
fi

if [ "$fails" -ne 0 ]; then
  echo "adr-registry-validate self-test FAILED ($fails)" >&2
  exit 1
fi
echo "PASS: adr-registry-validate self-test"
