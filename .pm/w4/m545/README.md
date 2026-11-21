# w4 · m545 — Ctrl-C in a sandbox terminal echoes `^C`, as on a Linux machine

**Worker:** worker4 **Goal:** a terminal inside a bex sandbox (an agent's PTY, the web Shell, any program a test drives on a PTY) echoes the interrupt, quit and suspend characters the way Linux's tty line discipline does, so programs and tests that rely on the kernel's echo behave as on a developer's machine or a CI runner. **Status:** todo

## Tasks (in order)

| id   | title                                                                  | est | depends_on |
| ---- | ---------------------------------------------------------------------- | --- | ---------- |
| t001 | Patch gVisor's line discipline to echo signal characters               | 1h  | —          |
| t002 | Send the patch upstream to gVisor                                      | 45m | t001       |
| t003 | Run a runsc carrying the fix on bex's sandbox nodes                    | 1h  | t001       |
| t004 | Live check: a production sandbox PTY echoes `^C`                       | 30m | t003       |
| t005 | Render parity: Render Sandboxes and Devin's terminals                  | 20m | t004       |
| t006 | Simplify                                                               | 15m | t005       |
| t007 | Test coverage                                                          | 30m | t005       |
| t008 | Closeout                                                               | 15m | t006, t007 |

## Definition of done

On a production bex sandbox:

- A fresh PTY in cooked mode (`ISIG ICANON ECHO ECHOCTL`) running `read` or `sleep`, sent `\x03`, shows `^C` on the screen and the foreground process gets SIGINT; `\x1c` shows `^\` with SIGQUIT and `\x1a` shows `^Z` with SIGTSTP.
- With `stty -echoctl` the same write echoes the raw byte; with `stty -echo` it echoes nothing (both as on Linux).
- beancount-io's five interrupt tests (`cli/tests/test_interrupt_contract.py`, 3 tests, and `cli/tests/test_native_interrupt_is_silent.py::test_ctrl_c_during_a_forwarded_program_exits_130_silently`, 2 params) pass inside a sandbox at an `origin/main` where they pass in GitHub CI.
- The sandbox nodes' runsc is pinned by sha512 in `infra/clusterapi/overlays/hetzner-caph/sandbox-pool.yaml`, and its binaries live outside git.

## Source + Goal linkage

- **Source:** 2026-11-20 root-cause research from the automation triage session. beancount-io's five interrupt tests failed 10/10 in agent sandboxes and passed in CI. An earlier run blamed libedit, but CI downloads the same `cpython-3.12.16-linux-x86_64-gnu` (uv `only-managed`, run 44846435392), and `bea` imports no readline. A controlled run in a production sandbox (`4.19.0-gvisor`, sandbox `sbx-dc0imgd2b0us73dobb40`, stopped afterwards) reproduced all five, failing only the `^C`-on-screen check (exit 130, no `Aborted!`, no traceback all held). A PTY probe showed `b''` under gVisor where Linux and macOS show `b'^C'`, with both the GNU-readline and libedit Pythons. Cause, in gVisor `pkg/sentry/fsimpl/devpts/line_discipline.go` (master ~:611-633; bex's pinned `release-20260622.0` ~:527-535): the VINTR/VSUSP/VQUIT branches signal the foreground group and drop the byte with no echo, while Linux's `n_tty_receive_signal_char` (`drivers/tty/n_tty.c` ~:1182-1192) echoes it under ECHO, as `^C` under ECHOCTL. gVisor's normal echo path also writes control characters raw; ECHOCTL is read only on the erase path. No gVisor issue reports it (closest: #422, closed 2023).
- **Goal linkage:** cloud coding-agent sessions ([ADR047](../../../docs/ADR047-cloud-coding-agent-sessions.md)) and sandboxes ([ADR104](../../../docs/ADR104-sandbox-render-parity.md)) run behind gVisor ([ADR042](../../../docs/ADR042-sandbox-cluster-substrate.md)); a terminal that differs from Linux makes a repository's own tests fail only in bex.
- **Expected outcome:** terminal-driven tests and interactive Ctrl-C behave in a sandbox as on a Linux machine; agents stop recording environment-only failures as repository bugs.
- **Why now:** scheduled agent routines run beancount-io's suite every few hours and keep tripping on these five tests; no runsc flag or newer gVisor release fixes it, so it needs a patch.
- **Render parity:** included (t005): sandbox and agent terminals are tenant-facing; Devin (the reference for agent surfaces) runs a real Linux kernel, whose terminals echo `^C`.
