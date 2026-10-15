#!/usr/bin/env bash
# Whole-program unreachable-code check over the lego/ workspace, run by
# `make lint`. Prints every unreachable lego/ function and exits 1 when there
# is one, or when an analysis fails.
#
#   bash hack/deadcode.sh <deadcode-binary>
#
# One analysis of backend + operator + cli together peaked at 8 GB and was
# OOM-killed on about half the CI runs (w1/m174), which silently turned the
# check off. The workspace import graph keeps a split from missing anything the
# single run caught: cli imports no other lego module and none imports it, and
# backend and operator never import each other, so each is analysed from its
# own roots and reports only its own packages (and their external _test
# packages). types is the one shared module: a types function is unreachable
# only if neither backend nor operator reaches it, so only the types findings
# both runs report are kept. Runs are sequential and GOMEMLIMIT makes each
# collect early; together they peak near 4 GB.
set -uo pipefail

deadcode="${1:?usage: hack/deadcode.sh <deadcode-binary>}"
cd "$(dirname "$0")/../.." || exit 1 # lego/, where the go.work lives
export GOMEMLIMIT="${GOMEMLIMIT:-3GiB}"
prefix='^github.com/bex-co/bex/lego'

# analyse <module> <reported packages regex>: the module's findings on stdout.
analyse() {
  local out status
  out="$("$deadcode" -tags=e2e -test -filter="$prefix/($2)(_test)?(/|$)" "./$1/..." 2>&1)"
  status=$?
  if [ "$status" -ne 0 ]; then
    echo "deadcode: the $1 analysis failed (exit $status)" >&2
    if [ "$status" -gt 128 ]; then
      echo "deadcode: killed by signal $((status - 128)); 9 is usually the out-of-memory killer" >&2
    fi
    printf '%s\n' "$out" >&2
    return 1
  fi
  printf '%s' "$out"
}

rc=0
backend="$(analyse backend 'backend|types')" || rc=1
operator="$(analyse operator 'operator|types')" || rc=1
cli="$(analyse cli cli)" || rc=1

findings="$(
  {
    grep -v '^types/' <<<"$backend"
    grep -v '^types/' <<<"$operator"
    printf '%s\n' "$cli"
    comm -12 <(grep '^types/' <<<"$backend" | sort) <(grep '^types/' <<<"$operator" | sort)
  } | sed '/^$/d'
)"
if [ -n "$findings" ]; then
  printf '%s\n' "$findings"
  rc=1
fi
exit "$rc"
