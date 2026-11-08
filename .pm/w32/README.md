# w32 — project workstream (worker32)

**Worker:** worker32. This is a general-purpose bex workstream. It may take work anywhere in the project; the milestones below are scheduled work and historical records, not a permanent purpose, specialty, or ownership boundary.

## Milestones

- [x] **m283** — [Sandbox network policy as on Render: allow-all by default, plus an HTTPS hostname allow-list](done/m283/README.md) (8 tasks) — **DONE 2026-11-08**: verified on production (isolation matrix 42482375396)
- [ ] **m284** — **BLOCKED (user: `scripts/qa-login.sh` fails with "no session established" on production — fix the QA credentials or the QA account's second factor, or supply a QA `BEX_ACCESS_TOKEN`; `/unblock w32`)** — [Render SDK v1.8.0 sandbox compatibility](blocked/m284/README.md) (7 tasks; t001 done, t003 static fixes deployed; t002 live walk next)
- [ ] **m285** — **BLOCKED (upstream: w2/m278 — its production walk must prove the rootfs pause/resume commit on production, and its pending user decision on memory/process loss also settles whether `--kind runtime` (Render's default) can be served or must be refused; `/unblock w2` / `/unblock w32`)** — [Sandbox filesystem snapshots, as on Render](blocked/m285/README.md) (8 tasks; none started)

## Inbox

- [x] [005](done/005.md) — `bex docs` opens GitHub source markdown instead of the public docs site — DONE
- [x] [001](done/001.md) — CLI installer fails with old cosign — DONE
- [x] [002](done/002.md) — Blueprint validate: unknown service type also reports a spurious `missing property 'ipAllowList'` — DONE
- [x] [003](done/003.md) — Static site wedges after a failed build: next deploy stays "Build queued", ipAllowList changes never apply — DONE
- [x] [004](done/004.md) — CLI skill's GitHub Actions example runs the installer on ubuntu-latest, which has no cosign — DONE
