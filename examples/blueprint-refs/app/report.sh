#!/bin/sh
# Writes one line per Blueprint reference this service was given, logs it, and
# serves it at GET / (w8/375). The literal markers are printed as they arrived.
# The generateValue and sync: false variables are secrets: only their length
# and shape are printed, never their values. It is POSIX sh, so busybox runs it.

# literal NAME VALUE SET: a declared marker, printed as resolved.
literal() {
  if [ -n "$3" ]; then printf '%s=%s\n' "$1" "$2"; else printf '%s unset\n' "$1"; fi
}

# secret NAME VALUE SET: the length and base64 shape only, never the value.
secret() {
  if [ -z "$3" ]; then
    printf '%s unset\n' "$1"
    return
  fi
  b64=no bytes=-
  if [ -n "$2" ] && printf '%s' "$2" | grep -Eq '^([A-Za-z0-9+/]{4})*([A-Za-z0-9+/]{2}==|[A-Za-z0-9+/]{3}=)?$'; then
    b64=yes
    bytes=$(printf '%s' "$2" | base64 -d | wc -c | tr -d ' ')
  fi
  printf '%s set length=%d base64=%s bytes=%s\n' "$1" "${#2}" "$b64" "$bytes"
}

dir=/tmp/blueprint-refs
mkdir -p "$dir"
{
  printf 'blueprint-refs report for %s\n' "${RENDER_SERVICE_NAME:-unknown}"
  literal GROUP_MARKER "${GROUP_MARKER-}" "${GROUP_MARKER+set}"
  literal PEER_MARKER "${PEER_MARKER-}" "${PEER_MARKER+set}"
  literal SITE_HOST "${SITE_HOST-}" "${SITE_HOST+set}"
  secret GENERATED_SECRET "${GENERATED_SECRET-}" "${GENERATED_SECRET+set}"
  secret MANUAL_SECRET "${MANUAL_SECRET-}" "${MANUAL_SECRET+set}"
} >"$dir/index.html"
cat "$dir/index.html"
# httpd as PID 1 would ignore SIGTERM and hold every stop for its grace period,
# so the shell stays PID 1 and passes the signal on.
httpd -f -p "${PORT:-10000}" -h "$dir" &
trap 'kill "$!"' TERM INT
wait
