# gVisor's PTY line discipline still differs from Linux outside signal echo

Why: a program that drives a terminal in a sandbox (a test harness, an editor, a REPL) gets a line discipline unlike the Linux one it gets on a developer machine or CI runner, and fails only in bex. w4/m545 fixed the echo of signal characters; these gaps remain in gVisor `pkg/sentry/fsimpl/devpts/line_discipline.go` (bex's `release-20260622.0-bex.1` and master alike), found while writing that fix.

- **No input flush on a signal character.** Linux's `isig()` (`drivers/tty/n_tty.c`) discards the pending input line and pending echoes unless `NOFLSH` is set, and notifies a packet-mode master (`TIOCPKT_FLUSHREAD`). gVisor keeps them, so `abc`, Ctrl-C, Enter reads `abc\n` instead of `\n`.
- **VKILL (Ctrl-U) is not implemented** in canonical mode. Linux erases the line, with ECHOK/ECHOKE deciding how. gVisor queues the byte, and since w4/m545 echoes it as `^U`.
- **VLNEXT (Ctrl-V) and VREPRINT (Ctrl-R)** are not implemented in canonical mode, and neither is ECHONL.
- **IXON flow control is not implemented.** Ctrl-S and Ctrl-Q reach the reader instead of stopping and starting output; since w4/m545 they are at least not echoed.
- **A disabled control character matches NUL.** With `stty intr undef` (VINTR = 0), a NUL byte sends SIGINT. Linux never treats `__DISABLED_CHAR` as special.

Each is small, and gVisor's syscall tests run against native Linux (`test/syscalls/linux/pty.cc`), so each fix carries its own test. Put them in the `bex-co/gvisor` fork on top of `bex/echo-signal-chars`, upstream them with the user's DCO sign-off as w4/m545 t002 does, and ship them to the sandbox pool by a later `release-20260622.0-bex.<n>` (or a stock release carrying them). Extend `scripts/pty-signal-echo-probe.py` for the ones a tenant would notice: the flush and VKILL.

Done when each gap above either has a fix whose test passes on native Linux and on bex's runsc, or is recorded in ADR104 as a known difference.
