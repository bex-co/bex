#!/usr/bin/env bash
# Self-test for scripts/stale-key-name-validate.sh. A gate that was never seen to
# fail proves nothing, so each rule has its own red or exempt case, the failure
# must name file:line, and the correct key name must pass.
set -euo pipefail

here="$(cd "$(dirname "$0")" && pwd)"
SCRIPT="$here/stale-key-name-validate.sh"
[ -x "$SCRIPT" ] || {
  echo "FAIL: $SCRIPT not executable" >&2
  exit 1
}

# The old name, assembled so this file does not match its own gate.
stale='id_b''ex'

fails=0
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

# fixture <dir> — a git repo whose every key name is the right one.
fixture() {
  local dir="$1"
  mkdir -p "$dir/docs" "$dir/.claude/skills/s" "$dir/.pm/w9/blocked" "$dir/.pm/w9/done/m1/done" "$dir/.github/workflows"
  git -C "$dir" init -q
  printf 'ssh -i ~/.ssh/bex "root@${CP_IP}" uptime\n' >"$dir/docs/runbook.md"
  printf 'BEX_SSH_PRIVATE_KEY_FILE=$HOME/.ssh/bex\n' >"$dir/.env.example"
  printf 'printf x > "$RUNNER_TEMP/bex-ssh/bex"\n' >"$dir/.github/workflows/w.yml"
  printf '# skill\n' >"$dir/.claude/skills/s/SKILL.md"
  printf '*.env\n.env\n' >"$dir/.gitignore"
  git -C "$dir" add -A
}

# assert <label> <want-rc> <dir> [expected-stderr-substring]
assert() {
  local label="$1" want="$2" dir="$3" needle="${4:-}" err got
  set +e
  err="$(STALE_KEY_ROOT="$dir" "$SCRIPT" 2>&1 >/dev/null)"
  got=$?
  set -e
  if [ "$got" -ne "$want" ]; then
    echo "FAIL: $label — exit $got, want $want" >&2
    printf '%s\n' "$err" | sed 's/^/        /' >&2
    fails=$((fails + 1))
    return
  fi
  if [ -n "$needle" ] && ! printf '%s' "$err" | grep -qF -- "$needle"; then
    echo "FAIL: $label — stderr did not name '$needle'" >&2
    printf '%s\n' "$err" | sed 's/^/        /' >&2
    fails=$((fails + 1))
    return
  fi
  echo "ok: $label (exit $got)"
}

new() {
  local d="$tmp/$1"
  fixture "$d"
  printf '%s' "$d"
}

d="$(new clean)"
assert "the right key name passes" 0 "$d"

d="$(new docs)"
printf 'intro\nssh -i ~/.ssh/%s "root@${CP_IP}" uptime\n' "$stale" >"$d/docs/runbook.md"
assert "a runbook with the old name fails and names file:line" 1 "$d" "docs/runbook.md:2"

d="$(new skill)"
printf 'BEX_SSH_KEY_PATH=~/.ssh/%s bash scripts/fetch-app-kubeconfig.sh /tmp/k\n' "$stale" >"$d/.claude/skills/s/SKILL.md"
assert "a skill with the old name fails" 1 "$d" ".claude/skills/s/SKILL.md:1"

d="$(new workflow)"
printf 'printf x > "$RUNNER_TEMP/bex-ssh/%s"\n' "$stale" >"$d/.github/workflows/w.yml"
assert "a workflow's temp key file with the old name fails" 1 "$d" ".github/workflows/w.yml:1"

d="$(new parked)"
printf 'needs ~/.ssh/%s on this machine\n' "$stale" >"$d/.pm/w9/blocked/002.md"
assert "a parked note with the old name fails" 1 "$d" ".pm/w9/blocked/002.md:1"

d="$(new open)"
printf 'needs ~/.ssh/%s\n' "$stale" >"$d/.pm/w9/003.md"
assert "an open note with the old name fails" 1 "$d" ".pm/w9/003.md:1"

d="$(new untracked)"
printf 'ssh -i ~/.ssh/%s host\n' "$stale" >"$d/docs/new.md"
assert "a file not yet added to git is scanned too" 1 "$d" "docs/new.md:1"

d="$(new 'done')"
printf 'no ~/.ssh/%s here (2026-08-28)\n' "$stale" >"$d/.pm/w9/done/001.md"
printf 'no ~/.ssh/%s here (2026-08-28)\n' "$stale" >"$d/.pm/w9/done/m1/done/t001.md"
assert "closed history under .pm/**/done/ is a dated record and passes" 0 "$d"

d="$(new ignored)"
printf 'KEY=~/.ssh/%s\n' "$stale" >"$d/.env"
assert "a gitignored file (.env) is never read" 0 "$d"

d="$(new message)"
printf 'x ~/.ssh/%s\n' "$stale" >"$d/docs/runbook.md"
# The message prints the tilde literally, so the pattern must not expand it.
# shellcheck disable=SC2088
want='~/.ssh/bex'
assert "the failure says what the key is" 1 "$d" "$want"

d="$tmp/notgit"
mkdir -p "$d"
assert "a directory that is not a git checkout is an error, not a pass" 2 "$d"

if [ "$fails" -ne 0 ]; then
  echo "$fails case(s) failed" >&2
  exit 1
fi
echo "all cases passed"
