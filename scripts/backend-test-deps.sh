#!/usr/bin/env bash
# backend-test-deps.sh — the backend suite's real dependencies, locally.
#
# Runs the Postgres 17, OpenFGA (seeded with the bex store and authz model) and
# OpenBao that .github/workflows/backend-test.yml runs — the very digests, read
# from that workflow so the two cannot drift — so `go test ./...` in
# lego/backend runs the integration tests instead of skipping them (w5/m112).
# Containers are named per checkout and Docker picks their host ports, so they
# never collide with a dev-N stack or with another worktree's run.
#
#   bash scripts/backend-test-deps.sh up     # start (idempotent), wait, seed
#   bash scripts/backend-test-deps.sh env    # print the exports for the tests
#   bash scripts/backend-test-deps.sh down   # remove the containers
#
#   bash scripts/backend-test-deps.sh up && eval "$(bash scripts/backend-test-deps.sh env)"
#   (cd lego/backend && GOWORK=off BEX_TEST_REQUIRE_DEPS=1 go test -p 1 ./...)
#
# -p 1 as in CI: the integration packages share one database and some reset it.
set -euo pipefail

root=$(cd "$(dirname "$0")/.." && pwd)
workflow="$root/.github/workflows/backend-test.yml"
key=$(printf '%s' "$root" | shasum | cut -c1-8)
prefix="bex-test-$key"
shim_dir="${TMPDIR:-/tmp}/$prefix/bin"
bao_token=backend-test-only
# Used only when the host has no promtool: CI's 2.55.1 line, digest-pinned.
promtool_image=prom/prometheus:v2.55.1@sha256:2659f4c2ebb718e7695cb9b25ffa7d6be64db013daba13e05c875451cf51b0d3

pin() { # the first <pattern>@sha256 digest in backend-test.yml
  local image
  image=$(grep -oE "$1@sha256:[0-9a-f]{64}" "$workflow" | head -1)
  [ -n "$image" ] || {
    echo "backend-test-deps: no $1 pin in $workflow" >&2
    exit 1
  }
  printf '%s' "$image"
}

running() { [ "$(docker inspect -f '{{.State.Running}}' "$1" 2>/dev/null)" = true ]; }

start() { # name, image, docker run args (before the image)..., -- image args...
  local name=$1 image=$2
  shift 2
  if running "$name" && [ "$(docker inspect -f '{{.Config.Image}}' "$name")" = "$image" ]; then
    return 0
  fi
  docker rm -f "$name" >/dev/null 2>&1 || true
  local opts=()
  while [ $# -gt 0 ] && [ "$1" != -- ]; do
    opts+=("$1")
    shift
  done
  [ $# -gt 0 ] && shift
  docker run -d --name "$name" "${opts[@]}" "$image" "$@" >/dev/null
}

port() { # the host port Docker mapped for <name>'s <container port>
  docker port "$1" "$2/tcp" | head -1 | sed 's/.*://'
}

wait_for() { # what, probe...
  local what=$1
  shift
  for _ in $(seq 1 90); do
    if "$@" >/dev/null 2>&1; then return 0; fi
    sleep 1
  done
  echo "backend-test-deps: $what did not become ready" >&2
  return 1
}

up() {
  local pg_image fga_image bao_image
  pg_image=${BEX_TEST_PG_IMAGE:-$(pin 'postgres:17')}
  fga_image=$(pin 'openfga/openfga:[^ @]+')
  bao_image=$(pin 'quay.io/openbao/openbao:[^ @]+')
  start "$prefix-postgres" "$pg_image" -e POSTGRES_PASSWORD=pw -p 127.0.0.1::5432
  start "$prefix-openbao" "$bao_image" -e BAO_DEV_ROOT_TOKEN_ID=$bao_token -p 127.0.0.1::8200 \
    -- server -dev -dev-listen-address=0.0.0.0:8200
  # In-memory and seeded from the current model on every up: never stale.
  docker rm -f "$prefix-openfga" >/dev/null 2>&1 || true
  start "$prefix-openfga" "$fga_image" -e OPENFGA_DATASTORE_ENGINE=memory -p 127.0.0.1::8080 -- run

  # Over TCP: the image's init-time server listens only on its unix socket.
  wait_for postgres docker exec "$prefix-postgres" pg_isready -h 127.0.0.1 -U postgres
  local fga bao
  fga="http://127.0.0.1:$(port "$prefix-openfga" 8080)"
  bao="http://127.0.0.1:$(port "$prefix-openbao" 8200)"
  wait_for openfga curl -sf "$fga/healthz"
  wait_for openbao curl -sf "$bao/v1/sys/health"
  local store_id
  store_id=$(curl -sf -X POST "$fga/stores" -H 'Content-Type: application/json' -d '{"name":"bex"}' | jq -r '.id')
  curl -sf -X POST "$fga/stores/$store_id/authorization-models" \
    -H 'Content-Type: application/json' -d @"$root/deploy/gitops/authz/model.json" >/dev/null

  if ! command -v promtool >/dev/null; then
    mkdir -p "$shim_dir"
    # promtool reads the rule file it is given; mount its directory.
    cat >"$shim_dir/promtool" <<EOF
#!/bin/sh
last=""; for a in "\$@"; do last=\$a; done
dir=\$(dirname "\$last")
exec docker run --rm -v "\$dir:\$dir" --entrypoint promtool $promtool_image "\$@"
EOF
    chmod +x "$shim_dir/promtool"
  fi
  echo "backend-test-deps: up; next: eval \"\$(bash scripts/backend-test-deps.sh env)\"" >&2
}

print_env() {
  running "$prefix-postgres" && running "$prefix-openfga" && running "$prefix-openbao" || {
    echo "backend-test-deps: not up; run: bash scripts/backend-test-deps.sh up" >&2
    exit 1
  }
  cat <<EOF
export BEX_TEST_DB_URI='postgres://postgres:pw@127.0.0.1:$(port "$prefix-postgres" 5432)/postgres?sslmode=disable'
export BEX_TEST_OPENFGA_URL='http://127.0.0.1:$(port "$prefix-openfga" 8080)'
export BEX_TEST_OPENBAO_KV_URL='http://127.0.0.1:$(port "$prefix-openbao" 8200)'
export BEX_TEST_OPENBAO_KV_TOKEN='$bao_token'
EOF
  if [ -x "$shim_dir/promtool" ] && ! command -v promtool >/dev/null; then
    echo "export PATH='$shim_dir':\"\$PATH\""
  fi
}

down() {
  docker rm -f "$prefix-postgres" "$prefix-openfga" "$prefix-openbao" >/dev/null 2>&1 || true
  rm -rf "${TMPDIR:-/tmp}/$prefix"
}

case "${1:-}" in
  up) up ;;
  env) print_env ;;
  down) down ;;
  *)
    echo "usage: $0 {up|env|down}" >&2
    exit 2
    ;;
esac
