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
"""Checks that a terminal handles signals, line editing, typeahead and flow control as Linux does.

The signal cases open a fresh PTY whose foreground process waits for a signal,
type one signal character into it, and check what the terminal shows and which
signal the process gets. In cooked mode Linux shows "^C" (w4/m545): gVisor's
line discipline showed nothing until bex's runsc carried the fix, so a
repository's interrupt tests failed only in a sandbox.

The line cases type a line at a process that reads one, and check what the
terminal shows and what the process reads (w4/m574, w4/m598). On Linux, Ctrl-C
throws away the line typed so far unless `stty noflsh` is set, Ctrl-U erases it
from the line and the screen, erasing a tab moves back to where the tab began,
and ECHOPRT echoes erased characters as `\\.../`.

The typeahead case types a complete line and a partial line while the foreground
process is not reading, then presses Ctrl-C (w4/m598): Linux echoes the
typeahead, delivers SIGINT at once, and flushes both lines.

The tcflow case checks TCXONC (TCOOFF/TCOON) output flow control (w4/m598).

Run it natively, under runsc, or in a bex sandbox. It prints one line per case
and exits 1 when any case differs from Linux.
"""

import errno
import fcntl
import os
import pty
import re
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

# name, termios local flags to set, the steps typed with the screen each shows,
# and the line the process reads. The process ignores SIGINT. ECHOE, ECHOK and
# ECHOKE are set as well, which Linux's tty_std_termios does, so Ctrl-U erases
# each character from the screen.
LINE_CASES = [
    ("ctrl-c flushes the line", 0, [(b"abc", b"abc"), (b"\x03", b"^C"), (b"\n", b"\r\n")], b"\n"),
    ("ctrl-c noflsh", termios.NOFLSH, [(b"abc", b"abc"), (b"\x03", b"^C"), (b"\n", b"\r\n")], b"abc\n"),
    ("ctrl-u erases the line", 0, [(b"abc", b"abc"), (b"\x15", b"\b \b" * 3), (b"def\n", b"def\r\n")], b"def\n"),
    ("tab erase", 0, [(b"a\t", b"a\t"), (b"\x7f", b"\b" * 7), (b"\n", b"\r\n")], b"a\n"),
    ("echoprt", termios.ECHOPRT, [(b"abc", b"abc"), (b"\x7f\x7f", b"\\cb"), (b"d\n", b"/d\r\n")], b"ad\n"),
]
ERASE_LFLAGS = termios.ECHOE | termios.ECHOK | termios.ECHOKE

READY = b"READY"
TIMEOUT = 10.0
# How long a step waits for its echo. A wrong echo is never complete.
STEP_TIMEOUT = 2.0
REPORT = re.compile(rb"GOT [0-9a-f]*\r\n")


def set_lflags(on, off):
    """Checks that the PTY starts in cooked mode, then changes its local flags."""
    attrs = termios.tcgetattr(0)
    if attrs[3] & COOKED != COOKED:
        os.write(1, b"NOT-COOKED %#o\n" % attrs[3])
        os._exit(2)
    attrs[3] = (attrs[3] | on) & ~off
    termios.tcsetattr(0, termios.TCSANOW, attrs)


def child(clear_lflags):
    """Runs in the PTY's session: wait for one signal, report it, exit."""
    set_lflags(0, clear_lflags)

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


def line_child(lflags):
    """Runs in the PTY's session: read one line, report it in hex, exit."""
    set_lflags(lflags | ERASE_LFLAGS, 0)
    signal.signal(signal.SIGINT, signal.SIG_IGN)
    os.write(1, READY + b"\n")
    line = os.read(0, 1024)
    os.write(1, b"GOT %s\n" % line.hex().encode())
    os._exit(0)


def run_line(lflags, steps, want_line):
    pid, fd = pty.fork()
    if pid == 0:
        line_child(lflags)
    deadline = time.monotonic() + TIMEOUT
    got = b"GOT %s\r\n" % want_line.hex().encode()
    screen = b""
    try:
        before = read_until(fd, lambda b: READY + b"\r\n" in b, deadline)
        if READY in before:
            # Each step is typed once the one before it is on the screen, as a
            # person types, so that each reaches the line discipline alone.
            for i, (typed, shows) in enumerate(steps):
                os.write(fd, typed)
                if i < len(steps) - 1:
                    done = lambda b, shows=shows: shows in b
                    step_deadline = min(deadline, time.monotonic() + STEP_TIMEOUT)
                else:
                    done = REPORT.search
                    step_deadline = deadline
                screen += read_until(fd, done, step_deadline)
    finally:
        if not REPORT.search(screen):
            os.kill(pid, signal.SIGKILL)
        _, status = os.waitpid(pid, 0)
        os.close(fd)
    if READY not in before:
        return False, "child not ready: %r" % before
    # The process reports what it read once the line ends, so on Linux the
    # report can land on either side of the newline's echo: compare what is
    # left without it.
    report = REPORT.search(screen)
    echo = screen.replace(report.group(0), b"", 1) if report else screen
    want_echo = b"".join(shows for _, shows in steps)
    ok = report is not None and report.group(0) == got and echo == want_echo and os.WIFEXITED(status) and os.WEXITSTATUS(status) == 0
    return ok, "screen %r (want %r and %r), child status %#x" % (screen, want_echo, got, status)


def typeahead_child():
    """Waits for SIGINT without reading stdin, then checks stdin is empty."""
    set_lflags(ERASE_LFLAGS, 0)
    sigs = {signal.SIGINT}
    signal.pthread_sigmask(signal.SIG_BLOCK, sigs)
    os.write(1, READY + b"\n")
    sig = signal.sigwait(sigs)
    fl = fcntl.fcntl(0, fcntl.F_GETFL)
    fcntl.fcntl(0, fcntl.F_SETFL, fl | os.O_NONBLOCK)
    try:
        leftover = os.read(0, 1024)
    except OSError as exc:
        if exc.errno != errno.EAGAIN:
            os.write(1, b"ERR %d\n" % exc.errno)
            os._exit(3)
        leftover = b""
    os.write(1, b"GOT %s %s\n" % (signal.Signals(sig).name.encode(), leftover.hex().encode()))
    os._exit(0)


def run_typeahead():
    pid, fd = pty.fork()
    if pid == 0:
        typeahead_child()
    deadline = time.monotonic() + TIMEOUT
    got = b"GOT SIGINT \r\n"
    steps = [(b"ls\n", b"ls\r\n"), (b"ab", b"ab"), (b"\x03", b"^C")]
    screen = b""
    try:
        before = read_until(fd, lambda b: READY + b"\r\n" in b, deadline)
        if READY in before:
            for i, (typed, shows) in enumerate(steps):
                os.write(fd, typed)
                if i < len(steps) - 1:
                    done = lambda b, shows=shows: shows in b
                    step_deadline = min(deadline, time.monotonic() + STEP_TIMEOUT)
                else:
                    done = lambda b: got in b and b"^C" in b
                    step_deadline = min(deadline, time.monotonic() + STEP_TIMEOUT)
                screen += read_until(fd, done, step_deadline)
    finally:
        if got not in screen:
            os.kill(pid, signal.SIGKILL)
        _, status = os.waitpid(pid, 0)
        os.close(fd)
    if READY not in before:
        return False, "child not ready: %r" % before
    echo = screen.replace(got, b"", 1)
    want_echo = b"".join(shows for _, shows in steps)
    ok = got in screen and echo == want_echo and os.WIFEXITED(status) and os.WEXITSTATUS(status) == 0
    return ok, "screen %r (want %r and %r), child status %#x" % (screen, want_echo, got, status)


def tcflow_child():
    """Stops and resumes replica output via tcflow(TCOOFF/TCOON)."""
    set_lflags(0, 0)
    fl = fcntl.fcntl(1, fcntl.F_GETFL)
    fcntl.fcntl(1, fcntl.F_SETFL, fl | os.O_NONBLOCK)
    try:
        termios.tcflow(1, termios.TCOOFF)
        try:
            os.write(1, b"BAD")
            stopped_ok = False
        except OSError as exc:
            stopped_ok = exc.errno == errno.EAGAIN
        termios.tcflow(1, termios.TCOON)
    except (OSError, termios.error) as exc:
        err = exc.errno if isinstance(exc, OSError) else exc.args[0]
        os.write(1, b"ERR %d\n" % err)
        os._exit(3)
    fcntl.fcntl(1, fcntl.F_SETFL, fl)
    os.write(1, b"GOT %s\n" % (b"OK" if stopped_ok else b"NOTSTOPPED"))
    os._exit(0)


def run_tcflow():
    pid, fd = pty.fork()
    if pid == 0:
        tcflow_child()
    deadline = time.monotonic() + TIMEOUT
    got = b"GOT OK\r\n"
    screen = b""
    try:
        screen = read_until(fd, lambda b: b"GOT " in b or b"ERR " in b, deadline)
    finally:
        if b"GOT " not in screen and b"ERR " not in screen:
            os.kill(pid, signal.SIGKILL)
        _, status = os.waitpid(pid, 0)
        os.close(fd)
    ok = screen == got and os.WIFEXITED(status) and os.WEXITSTATUS(status) == 0
    return ok, "screen %r (want %r), child status %#x" % (screen, got, status)


def main():
    failed = 0
    cases = (
        [(run, case) for case in CASES]
        + [(run_line, case) for case in LINE_CASES]
        + [(run_typeahead, ("canonical typeahead ctrl-c",))]
        + [(run_tcflow, ("tcflow TCOOFF/TCOON",))]
    )
    for check, (name, *args) in cases:
        ok, detail = check(*args)
        print("%s %s: %s" % ("ok  " if ok else "FAIL", name, detail), flush=True)
        failed += not ok
    print("%s: %d of %d cases differ from Linux" % (os.uname().release, failed, len(cases)))
    return 1 if failed else 0


if __name__ == "__main__":
    sys.exit(main())
