# w32 — project workstream (worker32)

**Worker:** worker32. This is a general-purpose bex workstream. It may take work anywhere in the project; the milestones below are scheduled work and historical records, not a permanent purpose, specialty, or ownership boundary.

## Milestones

- [x] **m283** — [Sandbox network policy as on Render: allow-all by default, plus an HTTPS hostname allow-list](done/m283/README.md) (8 tasks) — **DONE 2026-11-08**: verified on production (isolation matrix 42482375396)
- [ ] **m284** — **BLOCKED (user: `scripts/qa-login.sh` fails with "no session established" on production — fix the QA credentials or the QA account's second factor, or supply a QA `BEX_ACCESS_TOKEN`; `/unblock w32`)** — [Render SDK v1.8.0 sandbox compatibility](blocked/m284/README.md) (7 tasks; t001 done, t003 static fixes deployed; t002 live walk next)
- [ ] **m285** — **BLOCKED (upstream: w2/m278 — its production walk must prove the rootfs pause/resume commit on production, and its pending user decision on memory/process loss also settles whether `--kind runtime` (Render's default) can be served or must be refused; `/unblock w2` / `/unblock w32`)** — [Sandbox filesystem snapshots, as on Render](blocked/m285/README.md) (8 tasks; none started)
- [x] **m297** — [Sandbox environment variables, as on Render](done/m297/README.md) (9 tasks) — **DONE 2026-11-10**: verified on production (isolation matrix 42741929387) ← from `/pm-brainstorm more for w32` 2026-11-09, item 1
- [x] **m298** — [Sandbox execution history (`/sandboxes/{id}/execs`), as on Render](done/m298/README.md) (9 tasks) — **DONE 2026-11-10**: verified on production (isolation matrix 42741929387) ← from `/pm-brainstorm more for w32` 2026-11-09, item 2 (reopens executions from the ADR018 `—` row; groups stay a non-goal)
- [x] **m299** — [Remaining Render SDK gaps: relative remote paths and mint during resume](done/m299/README.md) (8 tasks) — **DONE 2026-11-10**: verified on production (isolation matrix 42741929387) ← from `/pm-brainstorm more for w32` 2026-11-09, item 3 (the m284 static review's residuals)

- [ ] **m348** — [A sandbox command runs under `bash -c` in the home directory, as on Render](m348/README.md) (9 tasks) ← `/qa-find-bugs-cli` loop sweep 865a 2026-11-12: `copy ./run.sh sbx:run.sh` then `exec -- sh run.sh` fails (`can't open 'run.sh'`, exec runs in `/`), `bash` is missing from the `alpine:3` base (exit 127), and busybox `sh` rejects bash syntax; the Render SDK passes the command to `bash -c` in the home directory; corrects w22/001; major

## Inbox

- [ ] [018](018.md) — A directory copied into a sandbox loses its files' modification times (1970), and uploads arrive owner-only (~45m) ← `/qa-find-bugs-cli` loop sweep dc0e 2026-11-12
- [ ] [017](017.md) — A sandbox ended by its lifetime timeout answers 404 for up to ten minutes, then reads `terminated` at the wrong time (~60m) ← `/qa-find-bugs-cli` loop sweep 4488 2026-11-12; the lifetime-expiry half of w3/m318's DoD
- [ ] [016](016.md) — A sandbox reads region `frankfurt` while its sandbox group reads `oregon` (~45m) ← `/qa-find-bugs-cli` loop sweep edb9 2026-11-12
- [ ] [015](015.md) — `bex sandboxes list` returns the oldest sandbox first; ADR104 D2 says newest first (~45m) ← `/qa-find-bugs-cli` loop sweep 865a 2026-11-12
- [x] [014](done/014.md) — `bex agent … -o json` ends with a blank line (the API's own trailing newline plus the CLI's) — DONE (the answer is trimmed before it is re-indented)
- [x] [013](done/013.md) — the CLI docs and ADR018 still list "Render" prose and the `pg create` wizard hint as branding residuals; w2/073 and w2/082 rewrote both — DONE (docs, ADR018 and the pin-bump checklist now describe the product-name rule and the wizard rewrite)
- [x] [012](done/012.md) — `BEX_WORKSPACE` set to a workspace name that starts with `tea-` is never resolved (sent as `ownerId`, "not allowed") — DONE (only a value with the id's whole shape is an id; `tea-house` is resolved as a name)
- [x] [011](done/011.md) — `bex agent … -o yaml` prints large integers as floats (`size: 2.097152e+06`) and changes 16-digit ones — DONE (every integer prints with the API's own digits)
- [x] [010](done/010.md) — `bex agent … --follow` exits 1 and prints no result when its last transcript read hits one transient error (429, 5xx) — DONE (that read is retried like the rest; if it still fails the command warns, prints the session and exits by its state)
- [x] [009](done/009.md) — `bex logout` and a bad `BEX_OUTPUT` name `RENDER_*` variables the user never set (`RENDER_API_KEY`, `RENDER_OUTPUT`) — DONE (the messages name `BEX_ACCESS_TOKEN` / `BEX_OUTPUT` when the launcher filled the upstream variable; a variable the user set keeps its name)
- [x] [008](done/008.md) — `bex ssh --help` and `bex ea blueprints --help` link Render's docs instead of Bex's — DONE (per-command links to the Bex pages; a test walks every help page)
- [x] [006](done/006.md) — List makes one Cilium read per allow-list sandbox — DONE (concurrent reads; no RBAC widening)
- [x] [007](done/007.md) — The Docker promtool fallback can't read the rule file — DONE

- [x] [005](done/005.md) — `bex docs` opens GitHub source markdown instead of the public docs site — DONE
- [x] [001](done/001.md) — CLI installer fails with old cosign — DONE
- [x] [002](done/002.md) — Blueprint validate: unknown service type also reports a spurious `missing property 'ipAllowList'` — DONE
- [x] [003](done/003.md) — Static site wedges after a failed build: next deploy stays "Build queued", ipAllowList changes never apply — DONE
- [x] [004](done/004.md) — CLI skill's GitHub Actions example runs the installer on ubuntu-latest, which has no cosign — DONE
