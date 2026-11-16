#!/usr/bin/env bash
# tracking-issue.sh — the one open/update/close path for a scheduled
# monitor's tracking issue (w6/m161). About twenty copies of label + list +
# edit-or-create + close lived in eleven workflows; this replaces them.
#
#   tracking-issue.sh open   --label L --title T --body-file F [--color C] [--description D]
#   tracking-issue.sh append --label L --title T --body-file F --line TEXT [--color C] [--description D]
#   tracking-issue.sh close  --label L --comment TEXT
#
# open keeps exactly one open issue per label: it edits the newest open one, or
# creates one. An edit rewrites the title as well as the body, so the title
# always names the current failure, not the first one (w4/687). A create is retried only after listing again, so a create that
# GitHub applied but whose response was lost cannot open a second issue.
# append adds TEXT as a line to the open issue's body, or opens one with F and
# TEXT, so each event stays listed until a person closes the issue.
# close closes the newest open issue with the label, if any. Every call goes
# through scripts/lib/gh-retry.sh; GITHUB_REPOSITORY names the repository.
set -euo pipefail
# shellcheck source=gh-retry.sh
. "$(dirname "${BASH_SOURCE[0]}")/gh-retry.sh"

usage() { echo "usage: $0 open|append --label L --title T --body-file F [--line TEXT] [--color C] [--description D] | close --label L --comment TEXT" >&2; exit 2; }
[ $# -ge 1 ] || usage
action=$1
shift
label='' title='' body_file='' color=B60205 description='' comment='' line=''
while [ $# -gt 0 ]; do
  [ $# -ge 2 ] || usage
  case "$1" in
    --label) label=$2 ;;
    --title) title=$2 ;;
    --body-file) body_file=$2 ;;
    --color) color=$2 ;;
    --description) description=$2 ;;
    --comment) comment=$2 ;;
    --line) line=$2 ;;
    *) usage ;;
  esac
  shift 2
done
[ -n "$label" ] || usage
: "${GITHUB_REPOSITORY:?GITHUB_REPOSITORY names the repository}"

open_issue() {
  gh_retry gh issue list --repo "$GITHUB_REPOSITORY" --label "$label" --state open \
    --json number --jq '.[0].number // empty'
}

# Edit the newest open issue with the label to $title and $body_file, or create one.
open_or_edit() {
  gh_retry gh label create "$label" --repo "$GITHUB_REPOSITORY" --color "$color" \
    ${description:+--description "$description"} --force
  existing=$(open_issue)
  if [ -n "$existing" ]; then
    gh_retry gh issue edit "$existing" --repo "$GITHUB_REPOSITORY" --title "$title" --body-file "$body_file"
    return 0
  fi
  for attempt in 1 2 3; do
    gh issue create --repo "$GITHUB_REPOSITORY" --title "$title" --label "$label" \
      --body-file "$body_file" && return 0
    [ "$attempt" = 3 ] && break
    sleep $((attempt * ${GH_RETRY_DELAY:-10}))
    # The failed create may have landed: never create a second one.
    existing=$(open_issue)
    if [ -n "$existing" ]; then
      gh_retry gh issue edit "$existing" --repo "$GITHUB_REPOSITORY" --title "$title" --body-file "$body_file"
      return 0
    fi
  done
  # The last create may have landed too.
  [ -n "$(open_issue)" ] && return 0
  echo "tracking-issue: could not open the $label issue" >&2
  return 1
}

case "$action" in
  open)
    [ -n "$title" ] && [ -r "$body_file" ] || usage
    open_or_edit
    ;;
  append)
    [ -n "$title" ] && [ -r "$body_file" ] && [ -n "$line" ] || usage
    body=$(mktemp)
    trap 'rm -f "$body"' EXIT
    existing=$(open_issue)
    if [ -n "$existing" ]; then
      gh_retry gh issue view "$existing" --repo "$GITHUB_REPOSITORY" --json body --jq .body >"$body"
    else
      cat "$body_file" >"$body"
    fi
    printf '%s\n' "$line" >>"$body"
    body_file=$body
    open_or_edit
    ;;
  close)
    [ -n "$comment" ] || usage
    existing=$(open_issue)
    [ -z "$existing" ] || gh_retry gh issue close "$existing" --repo "$GITHUB_REPOSITORY" --comment "$comment"
    ;;
  *) usage ;;
esac
