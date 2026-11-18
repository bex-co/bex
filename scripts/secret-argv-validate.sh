#!/usr/bin/env bash
# Production credentials never ride on a command line (w3/m472).
#
# `curl -H "Authorization: Bearer $HCLOUD_TOKEN"` puts the token in curl's argv,
# which every local user can read with `ps` (and /proc/<pid>/cmdline, and
# process accounting) while the request runs. The w3/m302 worker saw
# HCLOUD_TOKEN, which controls the whole Hetzner project, that way.
# scripts/lib/curl-auth.sh (`bex_curl_bearer NAME <curl args>`) hands it to curl
# on stdin instead.
#
# This gate fails when a tracked shell script passes a production-class name
# as the argument of -H/--header, -u/--user, --proxy-user or --oauth2-bearer
# (in a curl call, or an argument array built for one), and names each hit as
# file:line. The names are scripts/production-secrets.txt's, plus HCLOUD_TOKEN
# whether or not it is listed.
#
# Scope:
#   * every tracked *.sh. Comment lines are skipped, and so is the helper,
#     the one sanctioned place that builds the header.
#   * .github/workflows are not scanned. They run on ephemeral GitHub-hosted
#     runners, one job per VM, where any process that can read argv already
#     runs as the job's user with the secret in its environment, so argv adds
#     no exposure. (Moving infra.yml's calls would also start a production
#     Terraform apply.)
#   * the check is lexical. A secret first copied into another variable
#     (auth="Authorization: Bearer $HCLOUD_TOKEN"; curl -H "$auth") is not
#     seen: call the helper instead of building the header.
#
# Usage: bash scripts/secret-argv-validate.sh [<tree-ish>]
#   no argument  the working tree's tracked files (CI, local runs)
#   <tree-ish>   that commit's files; scripts/ship-gates.sh passes HEAD, so a
#                teammate's uncommitted edit never decides a push
# SECRET_ARGV_ROOT overrides the repository, so the self-test
# (scripts/secret-argv-validate.test.sh) can run against throwaway fixtures.
set -euo pipefail
cd "${SECRET_ARGV_ROOT:-$(dirname "$0")/..}"
rev="${1:-}"

list=scripts/production-secrets.txt
helper=scripts/lib/curl-auth.sh
if [ -n "$rev" ]; then
  table="$(git show "$rev:$list")" || {
    echo "error: cannot read $list at $rev" >&2
    exit 2
  }
else
  table="$(cat "$list")" || {
    echo "error: cannot read $list" >&2
    exit 2
  }
fi
names="$( { sed 's/#.*//' <<< "$table" | awk 'NF { print $1 }'; echo HCLOUD_TOKEN; } | sort -u | paste -sd'|' -)"
if [[ ! "$names" =~ ^[A-Za-z_][A-Za-z0-9_|]*$ ]]; then
  echo "error: $list holds a name that is not a shell variable name" >&2
  exit 2
fi

flag='(-[A-Za-z]*[Hu]|--header|--user|--proxy-user|--oauth2-bearer)'
# The flag's argument up to the expansion: unquoted characters and whole
# "quoted" words, then optionally a quote it is still inside.
arg="([^[:space:]'\"]|\"[^\"]*\")*(\"[^\"]*)?"
re="(^|[[:space:](])$flag([[:space:]]+|=)?$arg\\\$\\{?($names)([^A-Za-z0-9_]|\$)"

status=0
hits="$(git grep -nIE -e "$re" ${rev:+"$rev"} -- '*.sh' ":(exclude)$helper")" || status=$?
case "$status" in
  0) ;;
  1) exit 0 ;;
  *)
    echo "error: git grep failed (exit $status); is $PWD a git checkout?" >&2
    exit 2
    ;;
esac
# Drop the tree-ish prefix git grep adds, then comment lines.
hits="$(awk -v p="${rev:+$rev:}" '
  p != "" && index($0, p) == 1 { $0 = substr($0, length(p) + 1) }
  { text = $0; sub(/^[^:]*:[0-9]+:/, "", text); if (text ~ /^[[:space:]]*#/) next; print }
' <<< "$hits")"
[ -n "$hits" ] || exit 0

echo "FAIL: a production credential is passed on a command line, where ps shows it to every local user:" >&2
printf '%s\n' "$hits" | cut -c1-200 | sed 's/^/  /' >&2
cat >&2 << 'EOF'

Pass it to curl on stdin instead (w3/m472):
  source scripts/lib/curl-auth.sh
  bex_curl_bearer HCLOUD_TOKEN <curl args...>
The names checked are scripts/production-secrets.txt's, plus HCLOUD_TOKEN.
EOF
exit 1
