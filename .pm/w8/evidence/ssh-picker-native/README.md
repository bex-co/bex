# Current-pin native SSH picker evidence

Why: retain current-version command-handoff evidence for the documented upstream residual without creating a duplicate implementation issue.

This is supplementary coverage for [w6/done/020](../../../w6/done/020.md) and the [known SSH picker limitation](../../../../docs/cli-compatibility-checklist.md). No new issue is allocated, no implementation is scheduled, and neither the upstream client nor the launcher was patched.

On 2026-10-11, released **Bex v0.3.2** (VCS `a6350da0d6e451e063143432e06c548ba7b5851b`) and the unmodified **Render v2.27.0**, pin `a764810a768202704e7206eb7b87a47211fcd98e`, ran in real PTYs against a local loopback HTTP fixture. A PATH-local `ssh` recorder captured the exact child argv and exited without opening a connection. The dummy local API key is unrelated to QA credentials. Production APIs, paid resources and production SSH sessions were not used. The separately inspected customer installation remains Bex v0.2.1; these captures use the private released v0.3.2 binary.

| Native command and selection, each in both clients | Expected child target | Actual child target |
| --- | --- | --- |
| `bex ssh srv-12345678901234567890 -o interactive`; first instance | `srv-12345678901234567890-first111@ssh.example.invalid` | `srv-12345678901234567890@ssh.example.invalid` |
| Same command; second instance | `srv-12345678901234567890-second22@ssh.example.invalid` | `srv-12345678901234567890@ssh.example.invalid` |
| `bex ssh -o interactive`; select sole service, then first instance | `srv-12345678901234567890-first111@ssh.example.invalid` | `srv-12345678901234567890@ssh.example.invalid` |
| Service-ID command; Any instance | `srv-12345678901234567890@ssh.example.invalid` | Same, passes |
| `bex ssh srv-12345678901234567890-first111 -o interactive` | `srv-12345678901234567890-first111@ssh.example.invalid` | Same, passes |

Render uses the corresponding `render` executable and identical arguments. [native.json](native.json) contains all ten commands, selected terminal rows, child argv, TTY status, elapsed time, native exit and complete local requests/responses. All captures completed without timeout. Eight picker cases dismiss normally with exit 0; the two direct-instance controls end on the harness's explicit interrupt with exit -2. These are deliberate termination controls, not remote exit-status evidence.

The exact-pin producer has two callbacks in [cmd/ssh.go:70–75 and 86–91](https://github.com/render-oss/cli/blob/a764810a768202704e7206eb7b87a47211fcd98e/cmd/ssh.go#L70) that receive `instanceID` but assign `input.InstanceID = input.ServiceIDOrName`. The selection view returns the selected ID correctly in `pkg/tui/views/sshinstanceselect.go:54–98`; the consumer `pkg/tui/views/ssh.go:205–214` substitutes nonempty InstanceID into the SSH username. Consequently the selected replica is discarded before the child starts. The similarly spelled assignment in `cmd/ssh.go:48–50` is the correct direct-instance branch and must remain. Source review found the same two defective callbacks on upstream main `c4d106cfdaf8eec6d23475155e43f4baeeed2013`; the three narrow issue searches in [upstream-status.json](upstream-status.json) are not an exhaustive issue census and no upstream issue or PR was opened.

[fixtures.json](fixtures.json) preserves fake response shapes and the final service-list retry's complete request ledger. The per-case wire records in native.json cover the earlier eight cases as well. [actual-empty-retry-driver.py.txt](actual-empty-retry-driver.py.txt) is the actual final retry driver, which retains earlier successful cases from its existing results file. [reproducer.py.txt](reproducer.py.txt) derives a fresh ten-case run from that driver. To rerun locally, place the two same-version binaries at `release/bex` and `render-pin` beside a copied Python driver in a new private directory; run with Python 3. No real credentials are required. The driver owns its loopback server, recorder, temporary configs and child processes and removes them in finally blocks.

Earlier local harness attempts omitted `/v1/users` and then `/v1/projects`; those setup failures and missing-route panels are excluded. An initial recorder pass stopped parents before draining the terminal; its forced exit values are also excluded. The final driver drains terminal output until dismissal. The owned server, recorder and per-case configs are absent after the successful final run. No production fixture exists from this sweep.

This confirms native local target selection, not paid live wrong-pod execution, gateway behavior or an implemented fix. The direct instance-ID workaround remains documented. Any future fix belongs upstream: use the callback's selected ID in both picker branches, preserve empty Any selection and the correct direct-instance branch, then verify both entry paths with two choices and the controls. Do not fork or add a server workaround for the lost client-side argument.
