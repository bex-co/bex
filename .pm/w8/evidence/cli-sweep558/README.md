# CLI sweep 558 — w8/129–131 independent live verification after the pin flip

**Verdict: all three fixes independently confirmed live; no new bug.** Same isolated QA session (`puncsky@gmail.com`), `bex-canary`, CLI rebuilt at HEAD `77ec448ae`, 2026-10-19T13:13–13:17Z.

Deployment pin `73894ccea` advanced production to `d98d07f62`, past all three fixes (`df689525f`, `a190c54c5`, `5ae176831` all ancestors). The fixing worker had already live-checked and closed the three notes (`a96a36780`); this sweep is this loop's own second-route corroboration of the legs it originally filed, not a repeat of their fixture flights.

- **W8/129 (Postgres `role`/`suspenders`):** `bex postgres get dpg-db9r654flioc739mcpbg -o json` (pre-existing baseline, read-only) now returns `role: "primary"`, `suspenders: []`, `status: available` under `data` — previously this CLI read `role: ""` / `suspenders: null` against the pinned client's required-field schema.
- **W8/130 (repo transport):** the refusal remainder, re-probed live: `git@gitlab.com:…`, `ssh://git@bitbucket.org/…`, `git://github.com/…`, and port-qualified `ssh://git@github.com:22/…` all refuse at create with the named `400: repo must be an https URL, or a GitHub SSH URL (git@github.com:owner/repo.git or ssh://git@github.com/owner/repo.git)`; nothing was created (list stayed 8). The positive legs (both GitHub SSH spellings building over HTTPS to `live`, static publish plane) were live-proven in the fix's own acceptance recorded in `.pm/w8/done/130.md` and were not repeated.
- **W8/131 (Docker Hub early settle):** fresh owned free fixture `qa558-131recheck` (`srv-dbb1f27o17is73furqag`) with `docker.io/library/qa558-no-such-image-zz9:latest`. Deploy `dep-dbb1f27o17is73furqb0` created `13:13:44Z`, settled `update_failed` at `13:15:28Z` — **104 s**, against 15m06s pre-fix. `failureReason` carries the exact Docker Hub wording (`pull access denied, repository does not exist … insufficient_scope`) now classified permanent. Fixture deleted via CLI (`delete_exit=0`), list back to 8 with zero `qa558` names, REST `GET /services/{id}` → 404.

Baseline after the sweep: 8 services, no loop-owned leftovers. All four loop-filed bugs (w8/128–131) are now fixed, deployed, and live-verified.

## Unrun

- W8/130's positive SSH→HTTPS build flight (deliberately not repeated; two live proofs on record).
- Suspended-state `suspenders: ["user"]` on Postgres (would require mutating the baseline database; the fix's acceptance covered it on an owned fixture in another workspace).
