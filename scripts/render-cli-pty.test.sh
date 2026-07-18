#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
runner="$repo_root/scripts/render-cli-pty.py"
tmp="$(mktemp -d)"
trap 'rm -r "$tmp" 2>/dev/null || true' EXIT

fail() {
  echo "FAIL $*" >&2
  exit 1
}

assert_safe() {
  local path="$1"
  if grep -Fq 'pty-planted-secret-value' "$path"; then
    fail "planted secret reached durable runner output"
  fi
}

CI=true TERM=dumb RENDER_OUTPUT=json RENDER_CLI_CONFIG_PATH="$tmp/forbidden-config" \
  python3 "$runner" --timeout 5 --rows 37 --cols 111 \
  --expect ready=PTY_CHILD_READY --send-after $'ready=hello-child\n' \
  --expect done=PTY_CHILD_DONE -- \
  python3 -c '
import fcntl, os, struct, sys, termios
assert all(os.isatty(fd) for fd in (0, 1, 2))
assert os.environ.get("TERM") == "xterm-256color"
assert "CI" not in os.environ and "RENDER_OUTPUT" not in os.environ
assert "forbidden-config" not in os.environ["RENDER_CLI_CONFIG_PATH"]
rows, cols, _, _ = struct.unpack("HHHH", fcntl.ioctl(0, termios.TIOCGWINSZ, b"\0" * 8))
assert (rows, cols) == (37, 111)
print("PTY_CHILD_READY", flush=True)
assert sys.stdin.readline() == "hello-child\n"
print("PTY_CHILD_DONE", flush=True)
' >"$tmp/tty.out" 2>&1
grep -Fq 'PASS pty marker ready' "$tmp/tty.out" || fail "ready marker was not reported"
grep -Fq 'PASS pty marker done' "$tmp/tty.out" || fail "done marker was not reported"
grep -Fq 'PASS pty child-exit status=0' "$tmp/tty.out" || fail "zero exit was not reported"

set +e
python3 "$runner" --timeout 5 -- \
  python3 -c 'raise SystemExit(23)' >"$tmp/exit.out" 2>&1
exit_rc=$?
set -e
[[ "$exit_rc" == "23" ]] || fail "child exit 23 was not propagated (got $exit_rc)"
grep -Fq 'FAIL pty child-exit status=23' "$tmp/exit.out" || fail "non-zero exit was not named"

set +e
python3 "$runner" --timeout 5 --expect absent=NEVER_PRESENT -- \
  python3 -c 'print("pty-planted-secret-value", flush=True)' >"$tmp/redaction.out" 2>&1
marker_rc=$?
set -e
[[ "$marker_rc" == "1" ]] || fail "missing marker did not fail"
grep -Fq 'FAIL pty marker absent missing' "$tmp/redaction.out" || fail "missing marker was not named"
assert_safe "$tmp/redaction.out"

set +e
python3 "$runner" --timeout 0.4 --expect started=PTY_HANG_STARTED -- \
  python3 -c '
import pathlib, subprocess, sys, time
child = subprocess.Popen([
    sys.executable,
    "-c",
    "import signal,time; signal.signal(signal.SIGHUP, signal.SIG_IGN); time.sleep(60)",
])
pathlib.Path(sys.argv[1]).write_text(str(child.pid), encoding="ascii")
print("PTY_HANG_STARTED", flush=True)
time.sleep(60)
' "$tmp/descendant.pid" >"$tmp/timeout.out" 2>&1
timeout_rc=$?
set -e
[[ "$timeout_rc" == "124" ]] || fail "timeout returned $timeout_rc instead of 124"
grep -Fq 'FAIL pty timeout waiting-for=child-exit' "$tmp/timeout.out" || fail "timeout state was not named"
descendant_pid="$(<"$tmp/descendant.pid")"
for _ in {1..20}; do
  if ! kill -0 "$descendant_pid" 2>/dev/null; then
    descendant_pid=""
    break
  fi
  sleep 0.05
done
[[ -z "$descendant_pid" ]] || fail "timed-out descendant survived process-group teardown"

# Exercise the separate path where the PTY leader has already exited but one
# of its descendants still owns the terminal and must be killed at deadline.
set +e
python3 "$runner" --timeout 0.4 --expect started=PTY_ORPHAN_STARTED -- \
  python3 -c '
import os, pathlib, subprocess, sys, time
child = subprocess.Popen([
    sys.executable,
    "-c",
    "import pathlib,signal,sys,time; "
    "signal.signal(signal.SIGHUP, signal.SIG_IGN); "
    "pathlib.Path(sys.argv[1]).write_text('ready', encoding='ascii'); "
    "time.sleep(60)",
    sys.argv[2],
])
pathlib.Path(sys.argv[1]).write_text(str(child.pid), encoding="ascii")
deadline = time.monotonic() + 1
while not pathlib.Path(sys.argv[2]).exists() and time.monotonic() < deadline:
    time.sleep(0.01)
assert pathlib.Path(sys.argv[2]).exists()
print("PTY_ORPHAN_STARTED", flush=True)
os._exit(0)
' "$tmp/orphan.pid" "$tmp/orphan.ready" >"$tmp/orphan.out" 2>&1
orphan_rc=$?
set -e
[[ "$orphan_rc" == "124" ]] || fail "orphan timeout returned $orphan_rc instead of 124"
orphan_pid="$(<"$tmp/orphan.pid")"
for _ in {1..20}; do
  if ! kill -0 "$orphan_pid" 2>/dev/null; then
    orphan_pid=""
    break
  fi
  sleep 0.05
done
[[ -z "$orphan_pid" ]] || fail "orphaned PTY descendant survived process-group teardown"

set +e
python3 "$runner" --timeout 2 -- definitely-not-a-real-bex-client >"$tmp/missing.out" 2>&1
missing_rc=$?
set -e
[[ "$missing_rc" == "127" ]] || fail "missing binary returned $missing_rc instead of 127"
grep -Fq 'FAIL pty child-exit status=127' "$tmp/missing.out" || fail "missing binary exit was not named"

render_bin="${RENDER_BIN:-}"
if [[ -z "$render_bin" ]] && command -v render >/dev/null; then
  render_bin="$(command -v render)"
fi
if [[ -n "$render_bin" && -x "$render_bin" ]]; then
  set +e
  CI= TERM=xterm-256color RENDER_OUTPUT= \
    RENDER_HOST=http://127.0.0.1:1/v1/ RENDER_API_KEY=pty-planted-secret-value \
    RENDER_CLI_CONFIG_PATH="$tmp/non-tty-cli.yaml" \
    python3 - "$render_bin" <<'PY' >"$tmp/non-tty.out" 2>&1
import subprocess
import sys

try:
    result = subprocess.run(
        [sys.argv[1], "pgcli", "dpg-00000000000000000000"],
        stdin=subprocess.DEVNULL,
        capture_output=True,
        timeout=10,
        check=False,
    )
except (OSError, subprocess.TimeoutExpired):
    raise SystemExit(1)
if result.returncode == 0:
    raise SystemExit(1)
if b"`render pgcli` can only be used in interactive mode" not in result.stdout + result.stderr:
    raise SystemExit(1)
print("OFFICIAL_NON_TTY_GUARD_OK")
PY
  control_rc=$?
  set -e
  [[ "$control_rc" == "0" ]] || fail "piped official pgcli control missed the bounded guard"
  grep -Fq 'OFFICIAL_NON_TTY_GUARD_OK' "$tmp/non-tty.out" || fail "official guard marker is missing"
  assert_safe "$tmp/non-tty.out"
  echo "PASS official CLI non-TTY guard"
else
  echo "SKIP official CLI non-TTY guard (set RENDER_BIN to the pinned executable)"
fi

for output in "$tmp"/*.out; do
  assert_safe "$output"
done
echo "PASS render CLI PTY regression suite"
