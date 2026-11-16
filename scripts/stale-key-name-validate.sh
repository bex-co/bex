#!/usr/bin/env bash
# Stale production SSH key name gate (w21/089). The production key is ~/.ssh/bex:
# scripts/fetch-app-kubeconfig.sh uses it by default and .env.example names it
# BEX_SSH_PRIVATE_KEY_FILE. Runbooks, skills and notes kept calling it by an
# older name, a worker saw no such file, concluded "no production access" and
# parked three milestones on a gate that did not exist.
#
# Fails when the old name appears anywhere in the working tree and names each hit
# as file:line. The scan is `git grep` over tracked and untracked files, so it
# never reads gitignored files (.env, kubeconfigs) and sees a new file before it
# is committed. Exempt: .pm/**/done/**, the dated history of closed work, where a
# record of what an earlier worker saw or believed stays true as written.
#
# The name is assembled below from two pieces so that this script does not match
# its own pattern, and a plain `grep -rn` for the name finds nothing.
#
# STALE_KEY_ROOT is overridable so the self-test
# (scripts/stale-key-name-validate.test.sh) can run against throwaway fixture repos.
set -euo pipefail
cd "${STALE_KEY_ROOT:-$(dirname "$0")/..}"

stale='id_b''ex'

status=0
hits="$(git grep -nI --untracked -e "$stale" -- . ':(exclude,glob).pm/**/done/**')" || status=$?
case "$status" in
  1) exit 0 ;;
  0) ;;
  *)
    echo "error: git grep failed (exit $status); is $PWD a git checkout?" >&2
    exit 2
    ;;
esac

echo "FAIL: the stale production SSH key name \"$stale\" appears in:" >&2
printf '%s\n' "$hits" | cut -c1-200 | sed 's/^/  /' >&2
cat >&2 <<'EOF'

The production SSH key is ~/.ssh/bex (BEX_SSH_PRIVATE_KEY_FILE in .env.example;
scripts/fetch-app-kubeconfig.sh uses it by default, so most examples need no
BEX_SSH_KEY_PATH at all). A runbook that names a different file makes the next
reader conclude there is no production access: run the fetch before writing
that. Replace the name. Closed history under .pm/**/done/ is exempt.
EOF
exit 1
