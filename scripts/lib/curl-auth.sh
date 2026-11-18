# shellcheck shell=bash
# Send a bearer token to curl without putting it on a command line (w3/m472).
#
# `curl -H "Authorization: Bearer $TOKEN"` puts the token in curl's argv, which
# any local user can read with `ps` (and /proc/<pid>/cmdline, and process
# accounting) for as long as the request runs. The w3/m302 worker saw
# HCLOUD_TOKEN, which controls the whole Hetzner project, that way. This helper
# hands the header to `curl --config -` on stdin instead, as a here-string: a
# pipe on bash 5.1+, a 0600 temp file unlinked once opened on older bash. Either
# way only this user can read it, and no process's argv carries it. A
# here-string rather than `printf | curl`: bash fills it before curl starts, so
# a curl that exits without reading stdin cannot SIGPIPE the writer and fail
# the call under pipefail. scripts/secret-argv-validate.sh refuses the argv
# form for every production-class name.
#
# Usage (sourced):
#   bex_curl_bearer <VAR_NAME> <curl args...>
# It runs curl with "Authorization: Bearer <value of VAR_NAME>" and returns
# curl's exit status. curl's stdin carries the config, so the other arguments
# cannot also read stdin (`-d @-`, `-T -`).
bex_curl_bearer() {
  local name="$1" value
  shift
  value="${!name:-}"
  if [ -z "$value" ]; then
    echo "error: $name is empty" >&2
    return 1
  fi
  # A quote, backslash or line break would end or reinterpret the config line.
  case "$value" in
    *[\"\\]* | *$'\n'* | *$'\r'*)
      echo "error: $name contains a quote, backslash or line break; refusing to pass it to curl" >&2
      return 1
      ;;
  esac
  curl --config - "$@" <<< "header = \"Authorization: Bearer $value\""
}
