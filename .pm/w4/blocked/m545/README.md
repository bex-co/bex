# w4 · m545 — Ctrl-C in a sandbox terminal echoes `^C`, as on a Linux machine

**Worker:** worker4 **Goal:** a terminal inside a bex sandbox (an agent's PTY, the web Shell, any program a test drives on a PTY) echoes the interrupt, quit and suspend characters the way Linux's tty line discipline does, so programs and tests that rely on the kernel's echo behave as on a developer's machine or a CI runner. **Status:** blocked (t001, t003–t007 done; t002 needs the user's DCO sign-off; t008 follows)

## Tasks (in order)

| id   | title                                                                  | est | depends_on |
| ---- | ---------------------------------------------------------------------- | --- | ---------- |
| t001 | Patch gVisor's line discipline to echo signal characters — **DONE**               | 1h  | —          |
| t002 | Send the patch upstream to gVisor                                      | 45m | t001       |
| t003 | Run a runsc carrying the fix on bex's sandbox nodes — **DONE**                    | 1h  | t001       |
| t004 | Live check: a production sandbox PTY echoes `^C` — **DONE**                       | 30m | t003       |
| t005 | Render parity: Render Sandboxes and Devin's terminals — **DONE**                  | 20m | t004       |
| t006 | Simplify — **DONE**                                                               | 15m | t005       |
| t007 | Test coverage — **DONE**                                                          | 30m | t005       |
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

## Record 2026-11-21 (/loopx w4)

**Fix (t001).** Fork [bex-co/gvisor](https://github.com/bex-co/gvisor). One commit, `devpts: Echo signal characters and apply ECHOCTL to echoed control characters`, on two branches:

- `bex/echo-signal-chars` (`c69889c22`): master plus the fix, for upstream (t002).
- `bex/release-20260622.0-echo-signal-chars` (`c69b92ea5`): the same fix on the release bex pins, plus the fork's CI workflow.

In `pkg/sentry/fsimpl/devpts/line_discipline.go`, a signal character is echoed under ECHO, as `^C` with ECHOCTL and raw without it. The echo is queued before the signal, so it lands ahead of the process's own output. Other echoed control characters follow ECHOCTL, except TAB and a line's newline; in noncanonical mode a typed newline shows `^J`, as on Linux. Under IXON, START and STOP are not echoed.

Tests are the devpts unit test `TestEcho` plus eight syscall tests in `test/syscalls/linux/pty.cc`. Seven are `PtyTest.{SignalCharEchoedWithECHOCTL, SignalCharEchoedWithoutECHOCTL, SignalCharNotEchoedWithoutECHO, ControlCharEchoedWithECHOCTL, ControlCharEchoedWithoutECHOCTL, NoncanonNewlineEchoedWithECHOCTL, FlowControlCharsNotEchoedWithIXON}`. The eighth, `JobControlTest.SignalCharSignalsForegroundAndEchoes`, checks both the echo and the signal reaching a session's foreground group.

Fork CI checked the release branch in run [44956550082](https://github.com/bex-co/gvisor/actions/runs/44956550082) and master in run [44956550326](https://github.com/bex-co/gvisor/actions/runs/44956550326):

- The new tests pass on native Linux (kernel 7.0).
- The whole `pty_test_runsc_systrap_directfs` passes on the patched runsc.
- With only `line_discipline.go` reverted, six of the new tests fail on runsc. The two that describe what gVisor already did, `SignalCharNotEchoedWithoutECHO` and `ControlCharEchoedWithoutECHOCTL`, pass there too, as they should.

**Build and pin (t003).** Run [44955648773](https://github.com/bex-co/gvisor/actions/runs/44955648773) built `runsc` and `containerd-shim-runsc-v1` from `cd906399a` (the release branch at the time). It used `make copy`, the targets upstream's `make release` copies. The binaries are published as [release-20260622.0-bex.1](https://github.com/bex-co/gvisor/releases/tag/release-20260622.0-bex.1), outside git. Their SHA-512 digests are pinned in `infra/clusterapi/overlays/hetzner-caph/sandbox-pool.yaml`:

- runsc `2ef20fe6…2854438`; `runsc --version` reports `cd906399ac80`.
- shim `6670b239…2d4049a`.

The roll followed the overlay README, surge-first, on 2026-11-21:

1. Applied the `bex-sandbox` KubeadmConfigTemplate. `kubectl diff` showed only the URL and the two digests.
2. Scaled the pool to 2. `bex-sandbox-mzp4j-rsg2f` (cpx42) was Ready about 5 minutes later. On that node, `sha512sum` of both binaries matched the pins, and `runsc --version` reported `cd906399ac80`.
3. Cordoned `bex-sandbox-mzp4j-htm8z`. Its last agent sandbox (`ags-dc0vs8a5r8pc73e30h7g`) ended on its own, so no running session was cut off.
4. Marked that machine `delete-machine` and scaled back to 1.

The pool now has one node, which runs the pinned build. `capi-template-drift.sh` reports no drift.

**Live check (t004), QA user, workspace `bex-canary`, bex CLI v0.9.0:**

- **Before the roll, on the stock node.** In sandbox `sbx-dc11e6c6t83c738ncu30`, `scripts/pty-signal-echo-probe.py` found 4 of 5 cases differing from Linux: `^C`, `^\`, `^Z` and the raw `\x03` under `-echoctl` were never echoed. In sandbox `sbx-dc11fsr15tkc739a31s0` (image `python:3.12-bookworm`, imported as `qa-20261121-m545-py`), beancount-io at `07b0adb4b` failed all five interrupt tests with `the terminal never echoed the interrupt`. That commit is green in its GitHub CI (run of 2026-11-21 20:48).
- **After the roll, on the patched node.** Sandbox `sbx-dc11ngc6t83c738ncutg` was placed on `bex-sandbox-mzp4j-rsg2f`. The probe found 0 of 5 cases differing:
  - `^C`, `^\` and `^Z` were echoed, and SIGINT, SIGQUIT and SIGTSTP each reached the process.
  - Under `stty -echoctl` the raw `\x03` was echoed, and under `stty -echo` nothing was.
  - `sleep 30` and `sh -c read` both showed `^C`, and `sleep` died of SIGINT.

  At `07b0adb4b`, the three `test_interrupt_contract.py::test_ctrl_c_at_a_prompt_exits_130_silently` cases and both `test_native_interrupt_is_silent.py::test_ctrl_c_during_a_forwarded_program_exits_130_silently` cases passed, as did the other seven tests in those files.
- **Cleanup.** Every sandbox was stopped. `bex logout` ran and both Kratos sessions were revoked. The imported image `qa-20261121-m545-py` remains, because the CLI has no image delete.

**Render parity (t005).**

- **`bex sandboxes exec` has no TTY, and neither does Render's.** Render's [sandboxes-executing](https://render.com/docs/sandboxes-executing) page (read 2026-11-21) describes each command as starting "in a new shell" and streaming its output in chunks. It documents no pty, interactive terminal or stdin. The pinned CLI's `sandboxes exec` has no TTY flag either. So the PTYs a tenant meets are ones programs open inside the sandbox (devpts), which the fix covers.
- **The session web Shell and SSH terminal match Linux.** Both reach a sandbox through `pods/exec` with a TTY (`lego/backend/internal/sshgateway/exec.go`). That path was replayed against the QA sandbox's pod on the patched node: `sleep 30` was interrupted, the terminal showed `^C`, and the shell kept running (`echo STILL-$((1+1))` printed `STILL-2`).
- **Devin.** Devin, the reference for agent surfaces (ADR047), runs a real Linux kernel, whose terminals echo `^C`. No drift is left to file.

**Simplify (t006).** `/simplify` ran over the bex diff. It dropped an unused parameter in the probe; nothing else was found.

**Test coverage (t007).** `.github/workflows/sandbox-runsc-pty.yml` runs `scripts/sandbox-runsc-pty-check.sh` on every PR and main push that touches `sandbox-pool.yaml`, the probe or the check itself. The check runs the pool's own install item (its URL and pinned digests), then the probe natively and under the installed runsc. The fork's run [44957446419](https://github.com/bex-co/gvisor/actions/runs/44957446419) ran this check on `ubuntu-latest`:

- Against the stock pin from `origin/main`, it failed: 4 of 5 cases differed.
- Against the patched pin, it passed: 0 of 5 differed.

So a gVisor bump that loses the echo turns this check red before the pool rolls onto it. `scripts/clusterapi-validate.sh` now also accepts a `bex-co/gvisor` `release-<version>-bex.<n>` URL as a versioned gVisor source. It still refuses either `latest` channel, and fixtures prove both refusals.

**Line-discipline gaps outside this DoD:** filed as w4/712.

## Parked 2026-11-21

What is left is t002, which needs the user. gVisor's [CONTRIBUTING.md](https://github.com/gvisor/gvisor/blob/master/CONTRIBUTING.md) requires a DCO `Signed-off-by` on every commit. It also says "AI tools must not add `Signed-off-by` lines: only a human can certify the DCO", and that a human reviews AI-written work before submitting it. The steps for the user:

1. Review `bex-co/gvisor@bex/echo-signal-chars` (`c69889c22`).
2. Sign it off with `git commit --amend -s` and push it.
3. Open the pull request to `gvisor/gvisor`. The commit message is the description, and runs 44956550326 and 44956550082 are the evidence.

Then a worker records the PR link and status here (t002) and closes the milestone (t008). The DoD already holds on production, and the pool does not wait for upstream to merge.
