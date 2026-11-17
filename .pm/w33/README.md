# w33 — project workstream (worker33)

**Worker:** worker33. This is a general-purpose bex workstream. It may take work anywhere in the project; the milestones below are scheduled work and historical records, not a permanent purpose, specialty, or ownership boundary.

## Milestones

- [ ] **m435** — [A Blueprint's `initialDeployHook` runs once after the service's first successful deploy, beside `preDeployCommand`, as Render documents](blocked/m435/README.md) (11 tasks) ← from `/pm-brainstorm` for one aspect of render parity (Blueprints), 2026-11-16, item 1; the hook's failure rule is the proposal's choice, Render documents none — **BLOCKED (release gate, clears by itself: t001–t006 and t008–t010 are done and shipped in `d098bb097` (operator, bex-api, ADRs) and `c513acc1b` (dashboard); t011 replays the definition of done in `bex-canary` once a "pin platform images to" commit contains `c513acc1b`, checked with `git merge-base --is-ancestor c513acc1b <pinned sha>`. Also needed from the machine's owner for t007's local-cluster walk: raise the host's `fs.inotify.max_user_instances` from 128 (`sudo sysctl -w fs.inotify.max_user_instances=1024 fs.inotify.max_user_watches=1048576`, w33/005) and reprovision, or record through `/unblock w33` that the production replay stands in for it.)**
- [ ] **m436** — [A Blueprint Key Value with no `plan` is created on `256mb` (Starter), as Render's spec says](m436/README.md) (8 tasks) ← same brainstorm, item 2; a price-affecting default, recommended under ADR008, not a recorded user decision

## Inbox

- [ ] [001](001.md) — Run the render-examples corpus guard on today's 181 files and put it on the weekly drift schedule (~1h) ← same brainstorm, item 3
- [ ] [002](002.md) — Three Blueprint sentences in ADR006 and ADR049 contradict what bex ships (~30m) ← same brainstorm, item 4; docs only
- [ ] [003](003.md) — The deploy button falls back to the referring page when `repo` is absent, as Render documents (~45m) ← same brainstorm, item 5
- [ ] [004](004.md) — Capture what Render does with an `initialDeployHook` that fails, that is added late, and that outruns a timeout (~1h with a Render workspace) ← follow-up from w33/m435 t001 and t008, 2026-11-16; three of the hook's rules are bex's choices until captured
- [ ] [005](005.md) — `scripts/mock-cluster.sh` says so when the host's inotify limit cannot hold the cluster, instead of cycling workers forever (~1h) ← follow-up from w33/m435 t007, 2026-11-16; the local cluster has been half up for days on a host at the kernel default
