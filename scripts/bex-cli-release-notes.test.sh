#!/usr/bin/env bash
# Self-test for scripts/bex-cli-release-notes.sh: the rendered message lists
# only the CLI's own user-facing changes since the previous bex-cli release —
# never backend churn, never a wrong-prefix tag as the base — grouped for a
# reader, with install/upgrade/verify guidance and a compare link.
set -euo pipefail
here="$(cd "$(dirname "$0")" && pwd)"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
fails=0

export GIT_AUTHOR_NAME=t GIT_AUTHOR_EMAIL=t@example.com GIT_COMMITTER_NAME=t GIT_COMMITTER_EMAIL=t@example.com
export GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_NOSYSTEM=1

# repo <dir> — a history with a published base, a wrong-prefix tag, an
# abandoned higher orphan tag, and new user-facing + internal CLI commits.
repo() {
  local dir="$1"
  mkdir -p "$dir/scripts" "$dir/lego/cli" "$dir/lego/backend"
  cp "$here/bex-cli-release-notes.sh" "$dir/scripts/"
  git -C "$dir" init -q -b main
  echo a >"$dir/lego/cli/a.txt" && git -C "$dir" add -A &&
    git -C "$dir" commit -q -m "feat(cli): shipped command"
  git -C "$dir" tag bex-cli/v0.2.0
  echo b >"$dir/lego/backend/b.txt" && git -C "$dir" add -A &&
    git -C "$dir" commit -q -m "feat(backend): loud backend rework"
  git -C "$dir" tag bex-mobile/v0.5.0
  echo o >"$dir/lego/cli/o.txt" && git -C "$dir" add -A &&
    git -C "$dir" commit -q -m "feat(cli): abandoned orphan change"
  git -C "$dir" tag bex-cli/v0.9.9
  echo c >"$dir/lego/cli/c.txt" && git -C "$dir" add -A &&
    git -C "$dir" commit -q -m "feat(cli): bex s3 syncs a prefix"
  echo d >"$dir/lego/backend/d.txt" && git -C "$dir" add -A &&
    git -C "$dir" commit -q -m "fix(backend): quiet backend repair"
  echo e >"$dir/lego/cli/e.txt" && git -C "$dir" add -A &&
    git -C "$dir" commit -q -m "fix(cli): bex login retries once"
  echo f >"$dir/lego/cli/f.txt" && git -C "$dir" add -A &&
    git -C "$dir" commit -q -m "docs(cli): touch up help text"
}

# check <label> <want-rc> <needle> [<absent>] -- <script-args...>
# The fixture never tags the release under test (production triggers on the
# pushed tag, so it exists there); every call names the new ref explicitly.
check() {
  local label="$1" want="$2" needle="$3" absent="$4" out got
  shift 4
  set +e
  out="$(cd "$tmp/repo" && VERSION=0.2.1 bash scripts/bex-cli-release-notes.sh "$@" 2>&1)"
  got=$?
  set -e
  if [ "$got" -ne "$want" ] || ! printf '%s' "$out" | grep -qF "$needle" ||
    { [ -n "$absent" ] && printf '%s' "$out" | grep -qF "$absent"; }; then
    echo "FAIL: $label — exit $got (want $want): $out" >&2
    fails=$((fails + 1))
  else
    echo "ok: $label"
  fi
}

repo "$tmp/repo"
check "a new feat lands under What's new" 0 "bex s3 syncs a prefix" "" HEAD
check "a CLI fix lands under Fixes" 0 "bex login retries once" "" HEAD
check "backend churn is not listed" 0 "What's new" "loud backend rework" HEAD
check "the base is the previous bex-cli tag, not the orphan" 0 "compare/bex-cli/v0.2.0...bex-cli/v0.2.1" "0.9.9" HEAD
check "an unpublished orphan change ships now and is listed" 0 "abandoned orphan" "" HEAD
check "the wrong-prefix tag never moves the base" 0 "bex-cli/v0.2.0...bex-cli/v0.2.1" "bex-mobile" HEAD
check "internal CLI churn is counted, not bulleted" 0 "internal" "touch up help text" HEAD
check "install and upgrade guidance is present" 0 "bex upgrade" "" HEAD
check "the verify command is present" 0 "cosign verify-blob" "" HEAD
check "tap mode keeps the bex subject" 0 "bex 0.2.1" "" --tap HEAD
check "tap mode is plain text" 0 "What's new:" "#" --tap HEAD

# A pin bump is user-facing (it changes the CLI users run) even though its
# type is chore: it must be listed, not counted as internal.
pinrepo="$tmp/pin"
mkdir -p "$pinrepo/scripts" "$pinrepo/lego/cli"
cp "$here/bex-cli-release-notes.sh" "$pinrepo/scripts/"
git -C "$pinrepo" init -q -b main
echo v >"$pinrepo/lego/cli/go.mod" && git -C "$pinrepo" add -A &&
  git -C "$pinrepo" commit -q -m "feat(cli): first"
git -C "$pinrepo" tag bex-cli/v0.1.0
echo v2 >"$pinrepo/lego/cli/go.mod" && git -C "$pinrepo" add -A &&
  git -C "$pinrepo" commit -q -m "chore(deps): bump render-oss/cli to v9"
pin_out="$(cd "$pinrepo" && VERSION=0.1.1 bash scripts/bex-cli-release-notes.sh HEAD 2>&1)"
if ! printf '%s' "$pin_out" | grep -qF "bump render-oss/cli"; then
  echo "FAIL: an upstream pin bump is listed — got: $pin_out" >&2
  fails=$((fails + 1))
else
  echo "ok: an upstream pin bump is listed"
fi

# No CLI change at all still renders a releasable body, never an empty one.
emptyrepo="$tmp/empty"
mkdir -p "$emptyrepo/scripts" "$emptyrepo/lego/cli" "$emptyrepo/lego/backend"
cp "$here/bex-cli-release-notes.sh" "$emptyrepo/scripts/"
git -C "$emptyrepo" init -q -b main
echo a >"$emptyrepo/lego/cli/a.txt" && git -C "$emptyrepo" add -A &&
  git -C "$emptyrepo" commit -q -m "feat(cli): first"
git -C "$emptyrepo" tag bex-cli/v0.1.0
echo b >"$emptyrepo/lego/backend/b.txt" && git -C "$emptyrepo" add -A &&
  git -C "$emptyrepo" commit -q -m "feat(backend): only backend moves"
empty_out="$(cd "$emptyrepo" && VERSION=0.1.1 bash scripts/bex-cli-release-notes.sh HEAD 2>&1)"
if ! printf '%s' "$empty_out" | grep -qF "user-facing CLI changes"; then
  echo "FAIL: an empty range says so — got: $empty_out" >&2
  fails=$((fails + 1))
else
  echo "ok: an empty range says so"
fi

# A bad version and a missing tag fail instead of rendering the wrong notes.
set +e
(cd "$tmp/repo" && VERSION=nope bash scripts/bex-cli-release-notes.sh >/dev/null 2>&1)
[ $? -ne 0 ] || { echo "FAIL: a bad VERSION fails" >&2; fails=$((fails + 1)); }
(cd "$tmp/repo" && VERSION=0.4.0 bash scripts/bex-cli-release-notes.sh nope-ref >/dev/null 2>&1)
[ $? -ne 0 ] || { echo "FAIL: a missing new ref fails" >&2; fails=$((fails + 1)); }
set -e
echo "ok: bad inputs fail"

# A shallow clone (the default depth-1 checkout) cannot see the previous
# release, so it fails instead of rendering an empty changelog.
git clone -q --depth 1 "file://$tmp/repo" "$tmp/shallow"
set +e
shallow_out="$(cd "$tmp/shallow" && VERSION=0.2.1 bash scripts/bex-cli-release-notes.sh HEAD 2>&1)"
shallow_rc=$?
set -e
if [ "$shallow_rc" -eq 0 ] || ! printf '%s' "$shallow_out" | grep -qF "shallow clone"; then
  echo "FAIL: a shallow clone fails — exit $shallow_rc: $shallow_out" >&2
  fails=$((fails + 1))
else
  echo "ok: a shallow clone fails"
fi

[ "$fails" -eq 0 ] || exit 1
echo "PASS: bex-cli-release-notes"
