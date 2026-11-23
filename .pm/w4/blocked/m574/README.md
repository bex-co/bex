# w4 · m574 — A sandbox terminal edits and flushes typed input as Linux does

**Worker:** worker4 **Goal:** a PTY inside a bex sandbox handles typed input the way Linux's n_tty line discipline does, beyond the signal echo w4/m545 fixed. Ctrl-C throws away the line typed so far and Ctrl-U erases it. Ctrl-V, Ctrl-R and ECHONL work, Ctrl-S and Ctrl-Q stop and start output, and `stty intr undef` stops NUL from sending SIGINT. A program that drives a terminal (a test harness, an editor, a REPL) then behaves in a sandbox as it does on a developer machine or a CI runner. **Status:** blocked (t001–t006, t008–t012 done; t007 needs the user's DCO sign-off; t013 follows)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | A disabled control character is never special — **DONE** | 20m | — |
| t002 | A signal character flushes the pending input unless NOFLSH is set — **DONE** | 45m | t001 |
| t003 | VKILL erases the line in canonical mode, as ECHOK and ECHOKE say — **DONE** | 45m | t001 |
| t004 | VLNEXT, VREPRINT and ECHONL in canonical mode — **DONE** | 45m | t001 |
| t005 | IXON flow control: STOP holds output and START resumes it — **DONE** | 1h | t001 |
| t006 | Extend the PTY probe and the runsc CI check to the flush and VKILL — **DONE** | 30m | t002, t003 |
| t007 | Send the fixes upstream to gVisor | 30m | t001, t002, t003, t004, t005 |
| t008 | Build `release-20260622.0-bex.2`, pin it and roll the sandbox pool — **DONE** | 1h | t001, t002, t003, t004, t005, t006 |
| t009 | Live check: a production sandbox PTY flushes on Ctrl-C and erases on Ctrl-U — **DONE** | 30m | t008 |
| t010 | Render parity: Render Sandboxes and Devin's terminals — **DONE** | 20m | t009 |
| t011 | Simplify — **DONE** | 15m | t010 |
| t012 | Test coverage — **DONE** | 30m | t010 |
| t013 | Closeout | 15m | t011, t012 |

## Definition of done

- Each of the five gaps (t001–t005) has a syscall test in the fork's `test/syscalls/linux/pty.cc`. Each test passes on native Linux and on the patched runsc, and fails on runsc with only that fix reverted. A test that describes what gVisor already did may pass there too, and the README says which ones do. The fork's CI runs record all three.
- On a production bex sandbox, `scripts/pty-signal-echo-probe.py`, extended by t006, reports 0 cases differing from Linux. That includes w4/m545's five signal-echo cases and the new ones:
  - `abc`, Ctrl-C, Enter reads `\n`, and with `stty noflsh` it reads `abc\n`.
  - `abc`, Ctrl-U, `def`, Enter reads `def\n`, and the screen erases `abc` as Linux's default ECHOKE does.
- `infra/clusterapi/overlays/hetzner-caph/sandbox-pool.yaml` pins a `bex-co/gvisor` `release-20260622.0-bex.<n>` (or a stock release carrying every fix) by sha512, with the binaries outside git. Every sandbox node runs it.
- `.github/workflows/sandbox-runsc-pty.yml` fails against the `release-20260622.0-bex.1` pin and passes against the new one.
- Each fix has an upstream `gvisor/gvisor` pull request linked here (t007).
- A gap the milestone leaves unfixed is recorded in [ADR104](../../../../docs/ADR104-sandbox-render-parity.md) as a known difference, and in [ADR018](../../../../docs/ADR018-render-parity.md)'s sandbox rows as awaiting the user's sign-off.

## Source + Goal linkage

- **Source:** inbox note w4/712 ([`source-712.md`](source-712.md)), filed by w4/m545 t001 on 2026-11-21. Promoted by `/loopx w4` on 2026-11-22, because the work is five gVisor fixes, an upstream step, a fork release, a production pool roll and a live check, which is well over an hour across many tasks. w4/m545 took 8 tasks for one gap.
- **Premise checked 2026-11-22** in gVisor `pkg/sentry/fsimpl/devpts/line_discipline.go`, on `master` and on the fork's `bex/echo-signal-chars` (`c69889c22`):
  - **No flush.** The VINTR, VSUSP and VQUIT branches (master :611-632, fork :615-636) signal the group and `continue`. `q.readBuf` and the pending echoes are left in place.
  - **No VKILL.** A comment there says VKILL "isn't implemented" (master :637, fork :639).
  - **No VLNEXT, VREPRINT or ECHONL.** The file never mentions them.
  - **No IXON flow control.** Master has no IXON handling at all. The fork has only m545's echo suppression for START and STOP (`echoInput`, :801-805).
  - **NUL matches a disabled character.** With VINTR at 0, Go's `switch` matches a NUL byte against `case l.termios.ControlCharacters[linux.VINTR]`. EOF and EOL are already safe: `pkg/abi/linux/tty.go` checks `disabledChar` (:24-26) in `IsTerminating` (:129) and `IsEOF` (:141).
- **Goal linkage:** cloud coding-agent sessions ([ADR047](../../../../docs/ADR047-cloud-coding-agent-sessions.md)) and sandboxes ([ADR104](../../../../docs/ADR104-sandbox-render-parity.md)) run behind gVisor ([ADR042](../../../../docs/ADR042-sandbox-cluster-substrate.md)). A line discipline unlike Linux's makes a repository's own terminal tests fail only in bex. That is the failure w4/m545 fixed for `^C`.
- **Expected outcome:** after a Ctrl-C, a REPL or test harness sees the same input as on Linux. Ctrl-U edits a line, flow control works, and `stty intr undef` frees NUL. Agents stop recording these environment-only failures as repository bugs.
- **Why now:**
  - w4/m545 left the fork, its CI, the release build and the surge-first pool roll fresh and documented. One more release can carry all five fixes in a single roll.
  - The user is already asked to sign off m545's upstream patch, so these can be reviewed in the same sitting.
  - The flush and VKILL are what a tenant typing at a prompt notices first.
- **Render parity:** included (t010). Sandbox and agent terminals face tenants. Devin, the reference for agent surfaces, runs a real Linux kernel, whose n_tty behaves as this milestone's DoD says. Render's `sandboxes exec` has no TTY (w4/m545 t005), so the PTYs a tenant meets are the ones programs open inside the sandbox. These fixes cover them.

## Record 2026-11-22 (/loopx w4)

**Fixes (t001–t005).** Fork [bex-co/gvisor](https://github.com/bex-co/gvisor), five commits stacked on w4/m545's `devpts: Echo signal characters…`, each with its tests in `test/syscalls/linux/pty.cc` and, where useful, the devpts unit test:

| Fix | Upstream branch `bex/line-discipline` | Release branch | New syscall tests |
| --- | --- | --- | --- |
| `devpts: Never treat a disabled control character as special` (t001) | `e957a67ce` | `a99d10a24` | `DisabledChar*` (2) |
| `devpts: Flush input and output on a signal character unless NOFLSH is set` (t002) | `1f5fabdf2` | `e06027fff` | `SignalCharFlush*` (5) |
| `devpts: Erase the line on VKILL in canonical mode` (t003) | `6ef7a9160` | `43e88d7e0` | `KillChar*`, `EraseCharWithoutECHOE` (8) |
| `devpts: Implement VLNEXT, VREPRINT and ECHONL in canonical mode` (t004) | `09e1ab5f6` | `9861d5226` | `LiteralNext*`, `ReprintChar*`, `EchoNL*` (7) |
| `devpts: Implement IXON flow control` (t005) | `b66db2b09` | `b77cc2628` | `StopChar*` (9) |

`bex/line-discipline` (`b66db2b09`) is master plus m545's commit and these five, for upstream (t007). `bex/release-20260622.0-echo-signal-chars` (`44a08e148`) carries the same five on the release bex pins, then the fork's CI workflow, renamed `bex-line-discipline.yml`; `bex/line-discipline-ci` (`9397bd7ca`) is the upstream branch plus that workflow. What the fixes do, beyond the task files:

- **Disabled characters.** `KernelTermios.IsControlChar(c, i)` matches nothing for a disabled character, and every special character of the input path uses it (`IsEOF` and `IsTerminating` too).
- **The flush.** Input processing now collects echoes and writes them once it has processed a write, as n_tty does at the end of `n_tty_receive_buf()`, and sends signals after that, so the echo of `^C` still lands before the process's output. The flush then drops the echoes not written yet, as `isig()` does: writing `abc\x03\n` at once echoes `^C\r\n`. It also drops the output the master has no room for (pty.c's `pty_flush_buffer()` empties the master's flip buffer), and a packet mode master reads `TIOCPKT_FLUSHREAD | TIOCPKT_FLUSHWRITE`. Input processing cannot take `termiosMu` for writing or notify waiters, so it leaves packet status and replica events for `notifyInputEvents`, which runs once the locks are released.
- **VKILL** reuses the VERASE/VWERASE erase loop. VERASE without ECHOE now echoes the erase character, as `eraser()` does.
- **VLNEXT** keeps its state across writes and is forgotten when canonical mode is turned off. A literal newline does not end the line.
- **IXON.** While output is stopped, the replica is not writable and a write writes nothing, so a blocking writer waits and a nonblocking one gets `EAGAIN`; the echoes of input wait too. VSTART, a signal character, any character under IXANY, and clearing IXON start output again. `TIOCPKT_STOP` and `TIOCPKT_START` each replace the other. Flow control and signal characters are now handled before the other special characters, as in `n_tty_receive_char_special()`. `setTermios` and `setTermios2` share `updateTermios`.

**Verification, locally (the fork's CI could not run).** GitHub has disabled Actions on bex-co/gvisor: the pushes of 2026-11-23 03:05Z started no run, and a dispatch answers `422 Actions has been disabled for this repository` while the repository's Actions setting reads enabled. So the checks the fork's workflow encodes ran in docker on this machine (Linux 7.0.12, arm64, gVisor's systrap platform), with `pty_test` built by g++ against Debian's gtest and absl (one absl incompatibility in `test/util/mount_util.cc` patched in that copy only):

- **Native Linux:** the 41 new and m545 tests pass. Upstream's whole `pty_test` passes but `BasicPtyTest.SetMode`, which the container's devpts mount fails before and after these changes.
- **Patched runsc:** the whole `pty_test` passes, 144 of 144. On runsc with m545's fix only, every test that failed was a new one, so no existing test changed outcome.
- **Each fix reverted** (the line discipline and `tty.go` as they were before that fix): exactly the tests listed in the workflow fail. Four describe what gVisor already did and pass there too: `SignalCharFlushSkippedWithNOFLSH`, `KillCharInNoncanonicalMode`, `LiteralNextAndReprintCharInNoncanonicalMode` and `StopCharWithoutIXONIsOrdinary`.
- **The devpts unit test** passes, and `TestSignalFlushesOutputMasterHasNoRoomFor` fails without t002's fix. (That case is a unit test because on Linux how much output the master keeps depends on how far a worker thread got: 2,048 or 4,095 bytes in the native runs.)
- **checklocks** reports no lock errors on the master branch; its notes are the advisory kind master already prints.
- **A 71-case harness** of typed input, run natively and on runsc, now differs from Linux in 3 cases: erasing a tab and ECHOPRT (left as they were), and typeahead (below).

**The probe (t006).** `scripts/pty-signal-echo-probe.py` gains three line cases, typed one step at a time at a process that ignores SIGINT and reads one line: Ctrl-C reads `\n`, Ctrl-C under `noflsh` reads `abc\n`, and Ctrl-U reads `def\n` with each character erased from the screen. It reports 0 of 8 differing natively on Linux and on macOS 26.5 and on the patched runsc, and 2 of 8 (the flush and Ctrl-U) on m545's build. The workflow's paths are unchanged.

**Typeahead, found on the way:** while a complete line waits unread in canonical mode, gVisor processes no more input, so a line typed ahead is not echoed and a Ctrl-C after it does not interrupt a program that is not reading its terminal. Filed as w4/723 with the three differences this milestone leaves (tab erase, ECHOPRT, `tcflow()`), and recorded in [ADR104 § Terminals in a sandbox](../../../../docs/ADR104-sandbox-render-parity.md#terminals-in-a-sandbox-w4m545-w4m574) and ADR018's terminal row as awaiting the user's sign-off.

**Build (t008, first half).** Without the fork's CI, `runsc` was built with `go build` from gVisor's go branch, the tree upstream publishes for it: `d10071d63` is master at `d01b8319a`, four commits past the release tag, so their seven files go back to the tag's (every one of the tree's 1,440 shared Go files then equals the release branch's), and the fixes and two stateify fields are applied. Three builds, one with an empty cache and one from a fresh checkout plus the published patch, give the same digest. Published as [release-20260622.0-bex.2](https://github.com/bex-co/gvisor/releases/tag/release-20260622.0-bex.2), with `bex-go-tree.patch` and the build command in its notes:

- runsc `a907f14a…b807c96a`; `runsc --version` reports `44a08e148eae`.
- shim `6670b239…2d4049a`, bex.1's binary unchanged (it does not contain the line discipline).

**Pin and roll (t008).** `1a5a8e8723` pins `release-20260622.0-bex.2` in `sandbox-pool.yaml` (URL and runsc digest; the shim digest is unchanged). Its `sandbox-runsc-pty.yml` run [45109126999](https://github.com/bex-co/bex/actions/runs/45109126999) ran the pool's install item on `ubuntu-latest` (x86_64): `runsc version 44a08e148eae`, and the probe found 0 of 8 cases differing natively (Linux 7.0.0-1020-azure) and under that runsc. `clusterapi-validate.sh` and `capi-sandbox-bootstrap.test.sh` pass. The roll followed the overlay README, surge-first, on 2026-11-23 UTC:

1. Applied the `bex-sandbox` KubeadmConfigTemplate. `kubectl diff` showed only the URL and the runsc digest.
2. Scaled the pool to 2. `bex-sandbox-mzp4j-tssmk` (cpx42) was Ready about two minutes later. On it, `sha512sum` of both binaries matched the pins and `runsc --version` reported `44a08e148eae`.
3. Cordoned `bex-sandbox-mzp4j-rsg2f` at 04:02Z. One sandbox was running there, an agent session's in bex's own workspace (`tea-d98210cbbpdc73dcrkvg`), created at 04:00Z. It ended on its own at 04:30Z, so no running session was cut off.
4. Marked that machine `delete-machine` and scaled back to 1.

The pool now has one node, `bex-sandbox-mzp4j-tssmk`, which runs the pinned build. `capi-template-drift.sh` reports no drift.

**Live check (t009), QA user, workspace `bex-canary`, bex CLI v0.9.0.** Sandbox `sbx-dc1rmopfdpsc73e0hm8g` (image `qa-20261121-m545-py`, Python 3.12.16) was placed on `bex-sandbox-mzp4j-tssmk` and reports `4.19.0-gvisor`:

- `scripts/pty-signal-echo-probe.py` found 0 of 8 cases differing: m545's five, `ctrl-c flushes the line` (`abc^C\r\n`, the process read `0a`), `ctrl-c noflsh` (read `6162630a`) and `ctrl-u erases the line` (`abc\b \b\b \b\b \bdef\r\n`, read `6465660a`).
- The 71-case harness differed from native Linux in the same 3 cases as locally (tab erase, ECHOPRT, typeahead). Among the cases that match: with VINTR disabled a NUL echoed `^@` and was read; Ctrl-V Ctrl-C Enter echoed `^\b^C\r\n` and read `\x03\n`; after Ctrl-S a nonblocking replica write got `EAGAIN` and `poll` reported it not writable, a blocking write waited until Ctrl-Q, and Ctrl-U under the default flags echoed `\b \b` three times.
- beancount-io's interrupt suite was not rerun (the task's own condition): the flush changes what a typed line keeps, not the `^C` echo, the exit status or the output those five tests check.
- **Cleanup.** Both QA sandboxes (the first, `sbx-dc1rmh53vdpc73b0i91g`, was on the base image, which has no Python) were stopped and the canary sandbox namespace holds no pod. `bex logout` ran and the QA Kratos session was revoked.

**Render parity (t010).** Through `pods/exec` with a TTY, the path of the agent session's web Shell and the SSH terminal (`lego/backend/internal/sshgateway/exec.go`), a shell on the canary sandbox's pod on the patched node that ignored SIGINT and read one line showed `abc^C\r\n` and read an empty line for `abc`, Ctrl-C, Enter, and showed `abc\b \b\b \b\b \bdef\r\n` and read `def` for `abc`, Ctrl-U, `def`, Enter. Render's `sandboxes exec` has no TTY (m545 t005), and Devin runs a real Linux kernel, so Linux is the reference. The differences left are recorded in [ADR104 § Terminals in a sandbox](../../../../docs/ADR104-sandbox-render-parity.md#terminals-in-a-sandbox-w4m545-w4m574) and ADR018's terminal row as awaiting the user's sign-off, with the fix filed as w4/723.

**Simplify (t011).** `/simplify` ran over the bex diff and the fork's line discipline diff. In the probe, a parameter that shadowed its function's name was renamed and the results print as each case ends again. The fork diff already shares one disabled-character helper (`IsControlChar`) and one erase loop for VERASE, VWERASE and VKILL; nothing else was found. The probe passes again natively, on the patched runsc and fails the same two cases on m545's build.

**Test coverage (t012).** The runs above are the record: each fix's syscall tests fail with that fix reverted (exactly the listed ones; the four that describe what gVisor already did are named, and each test's comment says what Linux does); the devpts unit tests (`TestEcho` gained 18 cases, plus `TestFlowControl` and `TestSignalFlushesOutputMasterHasNoRoomFor`) fail without their fixes; the probe's new cases fail on m545's build (an arm64 build of the same source: runsc does not run under x86_64 emulation here, and the fork's `bex-probe.yml` could not run) and pass on bex.2 in run 45109126999 and in production.

## Parked 2026-11-23

What is left is t007, which needs the user, and t013, which closes the milestone after it. gVisor's [CONTRIBUTING.md](https://github.com/gvisor/gvisor/blob/master/CONTRIBUTING.md) requires a DCO `Signed-off-by` on every commit, says "AI tools must not add `Signed-off-by` lines: only a human can certify the DCO", and makes the contributor the sole author, so these commits carry no AI attribution either. The steps for the user, after w4/m545's own (its commit `c69889c22` is the base of this stack):

1. Review `bex-co/gvisor@bex/line-discipline` (`b66db2b09`): five commits on `c69889c22`, each message written as its pull request's description.
2. Sign them off (`git rebase --signoff c69889c22`, or `git rebase --signoff eb263e1f4` to sign m545's commit in the same pass) and push.
3. Open one pull request per fix to `gvisor/gvisor`, in order; `bex/line-discipline-1-disabled-chars` … `bex/line-discipline-5-ixon` point at each fix. The evidence is § Record 2026-11-22 above. To have the fork's CI record it as well, Actions must be re-enabled on bex-co/gvisor: GitHub disabled them there (a dispatch answers `422 Actions has been disabled for this repository` while the repository setting reads enabled), so that takes the org owner or GitHub support; `bex-line-discipline.yml` on `bex/line-discipline-ci` then runs every check above.

Then a worker records the links and their status here (t007) and closes the milestone (t013). The DoD's production half already holds (the pool does not wait for upstream to merge), except as § Record says for the fork's CI runs, which ran locally.
