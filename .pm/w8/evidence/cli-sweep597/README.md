# CLI sweep 597 — `ssh` dialects: non-TTY guard, free-plan ineligibility, synthetic targets

**Verdict: no bug filed.** Same isolated QA session, `bex-canary`, CLI rebuilt at `1a49cb806`, 2026-10-19. Owned fixture `qa597-web` (`srv-dbb9b3sgnrlc73bhss20`, free busybox), deleted: REST 404, zero `qa597` names. PTY probes via a small `pty` harness (`/tmp/qa501/pty597b.py`).

- **Non-TTY guard first:** every form (by id, by name, by instance id, synthetic, `--ephemeral`, `--plan` without `--ephemeral`) in a pipe fails identically: `` `bex ssh` can only be used in interactive mode ``, exit 1, before any server call. Upstream guard, not a server defect.
- **Free services honestly don't support SSH:** under a PTY, connecting to the live free fixture reaches "Preparing SSH connection…" then `service does not support ssh`. That is the eligibility gate working: SSH/`sshAddress` is paid-only ([service.go](/lego/backend/internal/apps/service.go:1624) — "paid, non-suspended web/private/worker"), so the free row carries no `sshAddress` and the pinned client refuses with that message. A positive SSH session needs a paid service, which this loop never creates; the positive path stays covered by `scripts/ssh-verify.sh` (w6/020).
- **Synthetic target under PTY:** the picker fails closed with `failed to list instances: 404 Not Found` — no hang, no panic.
- **Instance-id form under PTY** reaches the same support gate (no picker), consistent with the documented upstream picker defect (exact-instance selection in the menu is discarded at this pin; the direct argument is the working path — checklist, ssh-picker-native evidence).

## Unrun

- A real shell session (paid plan required).
- SSH to a suspended free service (same ineligibility expected; not separately probed).
