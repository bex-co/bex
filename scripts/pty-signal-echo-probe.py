#!/usr/bin/env python3
# Copyright 2026.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
"""Checks that a terminal echoes Ctrl-C, Ctrl-\\ and Ctrl-Z as Linux does.

Each case opens a fresh PTY whose foreground process waits for a signal, types
one signal character into it, and checks what the terminal shows and which
signal the process gets. In cooked mode Linux shows "^C" (w4/m545): gVisor's line
discipline showed nothing until bex's runsc carried the fix, so a repository's
interrupt tests failed only in a sandbox.

Run it natively, under runsc, or in a bex sandbox. It prints one line per case
and exits 1 when any case differs from Linux.
"""

import os
import pty
import select
import signal
import sys
import termios
import time

# Fresh PTYs are in cooked mode with these local flags set (Linux's
# tty_std_termios; macOS's defaults too).
COOKED = termios.ISIG | termios.ICANON | termios.ECHO | termios.ECHOCTL

# name, typed character, termios local flags to clear, expected echo, signal.
CASES = [
    ("ctrl-c", b"\x03", 0, b"^C", signal.SIGINT),
    ("ctrl-backslash", b"\x1c", 0, b"^\\", signal.SIGQUIT),
    ("ctrl-z", b"\x1a", 0, b"^Z", signal.SIGTSTP),
    ("ctrl-c -echoctl", b"\x03", termios.ECHOCTL, b"\x03", signal.SIGINT),
    ("ctrl-c -echo", b"\x03", termios.ECHO, b"", signal.SIGINT),
]

READY = b"READY"
TIMEOUT = 10.0


def child(clear_lflags):
    """Runs in the PTY's session: wait for one signal, report it, exit."""
    attrs = termios.tcgetattr(0)
    if attrs[3] & COOKED != COOKED:
        os.write(1, b"NOT-COOKED %#o\n" % attrs[3])
        os._exit(2)
    attrs[3] &= ~clear_lflags
    termios.tcsetattr(0, termios.TCSANOW, attrs)

    # Blocked and then waited for, so a signal that arrives before the wait
    # is not lost (a handler can run before a sleep starts, then sleep on).
    sigs = {signal.SIGINT, signal.SIGQUIT, signal.SIGTSTP}
    signal.pthread_sigmask(signal.SIG_BLOCK, sigs)
    os.write(1, READY + b"\n")
    sig = signal.sigwait(sigs)
    os.write(1, b"GOT %s\n" % signal.Signals(sig).name.encode())
    os._exit(0)


def read_until(fd, done, deadline):
    out = b""
    while not done(out) and time.monotonic() < deadline:
        ready, _, _ = select.select([fd], [], [], max(0.0, deadline - time.monotonic()))
        if not ready:
            break
        try:
            chunk = os.read(fd, 1024)
        except OSError:  # EIO once the child is gone.
            break
        if not chunk:
            break
        out += chunk
    return out


def run(char, clear_lflags, want_echo, want_sig):
    pid, fd = pty.fork()
    if pid == 0:
        child(clear_lflags)
    deadline = time.monotonic() + TIMEOUT
    got = b"GOT %s\r\n" % want_sig.name.encode()
    after = b""
    try:
        before = read_until(fd, lambda b: READY + b"\r\n" in b, deadline)
        if READY in before:
            os.write(fd, char)
            after = read_until(fd, lambda b: got in b, deadline)
    finally:
        if got not in after:
            os.kill(pid, signal.SIGKILL)
        _, status = os.waitpid(pid, 0)
        os.close(fd)
    if READY not in before:
        return False, "child not ready: %r" % before
    # The process prints its report after the signal, so on Linux it can
    # land on either side of the echo: compare what is left without it.
    echo = after.replace(got, b"", 1)
    ok = got in after and echo == want_echo and os.WIFEXITED(status) and os.WEXITSTATUS(status) == 0
    return ok, "screen %r (want %r and %r), child status %#x" % (after, want_echo, got, status)


def main():
    failed = 0
    for name, *case in CASES:
        ok, detail = run(*case)
        print("%s %s: %s" % ("ok  " if ok else "FAIL", name, detail))
        failed += not ok
    print("%s: %d of %d cases differ from Linux" % (os.uname().release, failed, len(CASES)))
    return 1 if failed else 0


if __name__ == "__main__":
    sys.exit(main())
