# w4 · m574 — A sandbox terminal edits and flushes typed input as Linux does

**Worker:** worker4 **Goal:** a PTY inside a bex sandbox handles typed input the way Linux's n_tty line discipline does, beyond the signal echo w4/m545 fixed. Ctrl-C throws away the line typed so far and Ctrl-U erases it. Ctrl-V, Ctrl-R and ECHONL work, Ctrl-S and Ctrl-Q stop and start output, and `stty intr undef` stops NUL from sending SIGINT. A program that drives a terminal (a test harness, an editor, a REPL) then behaves in a sandbox as it does on a developer machine or a CI runner. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | A disabled control character is never special | 20m | — |
| t002 | A signal character flushes the pending input unless NOFLSH is set | 45m | t001 |
| t003 | VKILL erases the line in canonical mode, as ECHOK and ECHOKE say | 45m | t001 |
| t004 | VLNEXT, VREPRINT and ECHONL in canonical mode | 45m | t001 |
| t005 | IXON flow control: STOP holds output and START resumes it | 1h | t001 |
| t006 | Extend the PTY probe and the runsc CI check to the flush and VKILL | 30m | t002, t003 |
| t007 | Send the fixes upstream to gVisor | 30m | t001, t002, t003, t004, t005 |
| t008 | Build `release-20260622.0-bex.2`, pin it and roll the sandbox pool | 1h | t001, t002, t003, t004, t005, t006 |
| t009 | Live check: a production sandbox PTY flushes on Ctrl-C and erases on Ctrl-U | 30m | t008 |
| t010 | Render parity: Render Sandboxes and Devin's terminals | 20m | t009 |
| t011 | Simplify | 15m | t010 |
| t012 | Test coverage | 30m | t010 |
| t013 | Closeout | 15m | t011, t012 |

## Definition of done

- Each of the five gaps (t001–t005) has a syscall test in the fork's `test/syscalls/linux/pty.cc`. Each test passes on native Linux and on the patched runsc, and fails on runsc with only that fix reverted. A test that describes what gVisor already did may pass there too, and the README says which ones do. The fork's CI runs record all three.
- On a production bex sandbox, `scripts/pty-signal-echo-probe.py`, extended by t006, reports 0 cases differing from Linux. That includes w4/m545's five signal-echo cases and the new ones:
  - `abc`, Ctrl-C, Enter reads `\n`, and with `stty noflsh` it reads `abc\n`.
  - `abc`, Ctrl-U, `def`, Enter reads `def\n`, and the screen erases `abc` as Linux's default ECHOKE does.
- `infra/clusterapi/overlays/hetzner-caph/sandbox-pool.yaml` pins a `bex-co/gvisor` `release-20260622.0-bex.<n>` (or a stock release carrying every fix) by sha512, with the binaries outside git. Every sandbox node runs it.
- `.github/workflows/sandbox-runsc-pty.yml` fails against the `release-20260622.0-bex.1` pin and passes against the new one.
- Each fix has an upstream `gvisor/gvisor` pull request linked here (t007).
- A gap the milestone leaves unfixed is recorded in [ADR104](../../../docs/ADR104-sandbox-render-parity.md) as a known difference, and in [ADR018](../../../docs/ADR018-render-parity.md)'s sandbox rows as awaiting the user's sign-off.

## Source + Goal linkage

- **Source:** inbox note w4/712 ([`source-712.md`](source-712.md)), filed by w4/m545 t001 on 2026-11-21. Promoted by `/loopx w4` on 2026-11-22, because the work is five gVisor fixes, an upstream step, a fork release, a production pool roll and a live check, which is well over an hour across many tasks. w4/m545 took 8 tasks for one gap.
- **Premise checked 2026-11-22** in gVisor `pkg/sentry/fsimpl/devpts/line_discipline.go`, on `master` and on the fork's `bex/echo-signal-chars` (`c69889c22`):
  - **No flush.** The VINTR, VSUSP and VQUIT branches (master :611-632, fork :615-636) signal the group and `continue`. `q.readBuf` and the pending echoes are left in place.
  - **No VKILL.** A comment there says VKILL "isn't implemented" (master :637, fork :639).
  - **No VLNEXT, VREPRINT or ECHONL.** The file never mentions them.
  - **No IXON flow control.** Master has no IXON handling at all. The fork has only m545's echo suppression for START and STOP (`echoInput`, :801-805).
  - **NUL matches a disabled character.** With VINTR at 0, Go's `switch` matches a NUL byte against `case l.termios.ControlCharacters[linux.VINTR]`. EOF and EOL are already safe: `pkg/abi/linux/tty.go` checks `disabledChar` (:24-26) in `IsTerminating` (:129) and `IsEOF` (:141).
- **Goal linkage:** cloud coding-agent sessions ([ADR047](../../../docs/ADR047-cloud-coding-agent-sessions.md)) and sandboxes ([ADR104](../../../docs/ADR104-sandbox-render-parity.md)) run behind gVisor ([ADR042](../../../docs/ADR042-sandbox-cluster-substrate.md)). A line discipline unlike Linux's makes a repository's own terminal tests fail only in bex. That is the failure w4/m545 fixed for `^C`.
- **Expected outcome:** after a Ctrl-C, a REPL or test harness sees the same input as on Linux. Ctrl-U edits a line, flow control works, and `stty intr undef` frees NUL. Agents stop recording these environment-only failures as repository bugs.
- **Why now:**
  - w4/m545 left the fork, its CI, the release build and the surge-first pool roll fresh and documented. One more release can carry all five fixes in a single roll.
  - The user is already asked to sign off m545's upstream patch, so these can be reviewed in the same sitting.
  - The flush and VKILL are what a tenant typing at a prompt notices first.
- **Render parity:** included (t010). Sandbox and agent terminals face tenants. Devin, the reference for agent surfaces, runs a real Linux kernel, whose n_tty behaves as this milestone's DoD says. Render's `sandboxes exec` has no TTY (w4/m545 t005), so the PTYs a tenant meets are the ones programs open inside the sandbox. These fixes cover them.
