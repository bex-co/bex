#!/usr/bin/env bash
# Sandbox substrate probe (docs/ADR105-agent-sandbox-hetzner-bare-metal.md,
# w6/m331 t001/t006). Runs the same four probes and the same build replay on
# every substrate ADR105 compares (gVisor on the cloud node, runc / runsc /
# each Kata VMM on the bare-metal node), so the numbers are comparable.
#
#   sandbox-probe.sh probes                 the four syscall/file probes
#   sandbox-probe.sh replay SRC             the routine replay over a bex source tree
#   sandbox-probe.sh replay-go SRC          its Go half (lego/)
#   sandbox-probe.sh replay-dashboard SRC   its dashboard half (yarn via corepack)
#   sandbox-probe.sh all SRC                probes, then the replay
#
# Output is one `step<TAB>seconds` line per timed step on stdout; progress goes
# to stderr. Run it inside the sandbox (kubectl exec / sandbox exec), not on
# the host.
#
# PROBE_DIR must be on the container root filesystem. Under gVisor `/tmp` is an
# internal tmpfs, which would measure memory instead of the rootfs path the
# agent's checkout uses (ADR042 D5), so the default is /var/tmp. The file
# probes use shell builtins only, so they time opens and writes, not process
# starts. The replay uses fresh GOCACHE / GOMODCACHE / yarn cache directories
# under PROBE_DIR: module and package downloads are timed as their own steps
# and are network-bound, so compare the build, vet, test and tsc steps.
set -euo pipefail

PROBE_DIR="${PROBE_DIR:-/var/tmp/sandbox-probe}"
mode="${1:-probes}"
src="${2:-}"

case "$mode" in
probes | replay | replay-go | replay-dashboard | all) ;;
*)
  echo "usage: $0 {probes|replay|replay-go|replay-dashboard|all} [SRC]" >&2
  exit 2
  ;;
esac
if [ "$mode" != probes ] && [ -z "$src" ]; then
  echo "error: $mode needs the bex source tree (with lego/ and dashboard/)" >&2
  exit 2
fi
if [ -z "${EPOCHREALTIME:-}" ]; then
  echo "error: bash 5+ is required (EPOCHREALTIME)" >&2
  exit 2
fi

now() { printf '%s' "${EPOCHREALTIME/,/.}"; }

# timed NAME CMD... — run CMD, print NAME and its wall seconds (3 decimals).
timed() {
  local name="$1" start end
  shift
  echo "· $name" >&2
  start=$(now)
  "$@" >&2
  end=$(now)
  awk -v n="$name" -v s="$start" -v e="$end" 'BEGIN { printf "%s\t%.3f\n", n, e - s }'
}

spawn_300() {
  local i bin
  bin=$(type -P true)
  for ((i = 0; i < 300; i++)); do "$bin"; done
}

create_2000() {
  local i
  mkdir -p "$PROBE_DIR/create"
  for ((i = 0; i < 2000; i++)); do printf 'x\n' >"$PROBE_DIR/create/f$i"; done
}

seed_3000() {
  local i
  mkdir -p "$PROBE_DIR/read"
  for ((i = 0; i < 3000; i++)); do printf 'package p\n' >"$PROBE_DIR/read/f$i.go"; done
}

read_3000() {
  local i
  for ((i = 0; i < 3000; i++)); do read -r _ <"$PROBE_DIR/read/f$i.go"; done
}

write_200m() {
  dd if=/dev/zero of="$PROBE_DIR/seq" bs=1M count=200 conv=fsync status=none
}

run_probes() {
  wipe "$PROBE_DIR"
  mkdir -p "$PROBE_DIR"
  timed spawn_300_processes spawn_300
  seed_3000
  timed read_3000_files read_3000
  timed create_2000_files create_2000
  timed write_200mb_sequential write_200m
  wipe "$PROBE_DIR"
}

# wipe DIR — remove a replay tree. Go writes its module cache read-only, so
# make it writable first or rm fails (and set -e ends the run).
wipe() {
  [ -e "$1" ] && chmod -R u+w "$1"
  rm -rf "$1"
}

replay_dir() {
  local work="$PROBE_DIR/replay"
  wipe "$work"
  mkdir -p "$work"
  printf '%s' "$work"
}

replay_go() {
  local work pkgs
  work=$(replay_dir)
  cp -a "$src/lego" "$work/"
  export GOCACHE="$work/gocache" GOMODCACHE="$work/gomodcache"
  (cd "$work/lego" && timed go_mod_download go mod download)
  # A go.work root holds no module of its own, so `./...` there matches
  # nothing; build every module the workspace lists.
  pkgs=$(cd "$work/lego" && go list -m -f '{{.Dir}}' | sed "s|^$work/lego|.|; s|\$|/...|")
  # shellcheck disable=SC2086 # one pattern per workspace module
  (cd "$work/lego" && timed go_build go build $pkgs)
  # shellcheck disable=SC2086
  (cd "$work/lego" && timed go_vet go vet $pkgs)
  (cd "$work/lego/types" && timed go_test_types go test ./...)
  wipe "$work"
}

replay_dashboard() {
  local work
  work=$(replay_dir)
  cp -a "$src/dashboard" "$work/"
  # The dashboard pins yarn 4 through `packageManager`; the image's global
  # yarn is 1.x, so go through corepack with a fresh home and no prompt.
  export COREPACK_HOME="$work/corepack" COREPACK_ENABLE_DOWNLOAD_PROMPT=0
  export YARN_CACHE_FOLDER="$work/yarncache" YARN_ENABLE_GLOBAL_CACHE=false
  (cd "$work/dashboard" && timed yarn_install corepack yarn install --immutable)
  (cd "$work/dashboard" && timed dashboard_typecheck corepack yarn typecheck)
  wipe "$work"
}

echo "sandbox-probe: kernel $(uname -r), $(getconf _NPROCESSORS_ONLN) cpus, dir $PROBE_DIR" >&2
case "$mode" in
probes) run_probes ;;
replay)
  replay_go
  replay_dashboard
  ;;
replay-go) replay_go ;;
replay-dashboard) replay_dashboard ;;
all)
  run_probes
  replay_go
  replay_dashboard
  ;;
esac
