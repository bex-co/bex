#!/usr/bin/env bash

# Shared helper for provisioning Kubernetes Secrets without placing secret bytes
# in a child process's argv. Callers pass values to this shell function; the
# only external process that handles the encoded data receives it on stdin.

secret_b64() {
  printf '%s' "$1" | base64 | tr -d '\n'
}

# secret_manifest NAMESPACE NAME TYPE KEY VALUE [KEY VALUE ...] — Secret YAML on stdout.
secret_manifest() {
  local namespace="$1" name="$2" type="$3"
  shift 3
  if (( $# == 0 || $# % 2 != 0 )); then
    printf 'secret-install: key/value pairs required for %s/%s\n' "$namespace" "$name" >&2
    return 2
  fi

  printf '%s\n' 'apiVersion: v1' 'kind: Secret' 'metadata:'
  printf '  name: %s\n  namespace: %s\n' "$name" "$namespace"
  printf 'type: %s\n' "$type"
  printf '%s\n' 'data:'
  while (( $# > 0 )); do
    local key="$1" value="$2"
    shift 2
    printf '  %s: %s\n' "$key" "$(secret_b64 "$value")"
  done
}

# apply_secret NAMESPACE NAME TYPE KEY VALUE [KEY VALUE ...]
apply_secret() {
  local manifest
  manifest="$(secret_manifest "$@")" || return
  printf '%s\n' "$manifest" | kubectl apply -f - >/dev/null
}
