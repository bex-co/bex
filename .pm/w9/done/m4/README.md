# w9 · m4 — cli-compat verify completeness: guard every fixed RC

**Worker:** worker9 **Goal:** `scripts/cli-compat.sh verify` re-runs every ✅ row of `docs/cli-compatibility-checklist.md` that makes a bex-api call — postgres lifecycle, services CRUD, deploys, logs, environments, logout join the existing keyvalues/instances legs — so none of the fifteen fixed root causes (RC2, RC7–RC12 especially) can silently regress. **Status:** done — one green 52-assertion `scripts/cli-compat.sh verify` run recorded against bex-api `a4771886`, all six families mutation-proven non-vacuous (`scripts/cli-compat.sh mutation-check`); see [evidence.md](evidence.md).

## Tasks (in order)

| id   | title                                                                                                          | est | depends_on              | status     |
| ---- | ---------------------------------------------------------------------------------------------------------------- | --- | ----------------------- | ---------- |
| t001 | Postgres lifecycle legs: create (with `--ip-allow-list`) · list/get-by-name · update flags (`--plan`/`--disk-size-gb`/`--high-availability`/`--ip-allow-list`/`--clear-ip-allow-list`) · rename · suspend/resume · delete | 45m | —                       | — **DONE** |
| t002 | Services create/update/delete + deploys list/create/cancel + logs legs                                             | 45m | —                       | — **DONE** |
| t003 | Environments + real device-token logout leg; update the checklist's Reproducing section to state exact coverage    | 30m | t001, t002, w4/m27/t005 | — **DONE** |
| t004 | Simplify — `/simplify` over the verify-script diff                                                                 | 20m | t003                    | — **DONE** |
| t005 | Test coverage — prove the legs fail loudly: mutation checks against a broken response shape                        | 30m | t003                    | — **DONE** |
| t006 | Closeout — DoD met → move milestone to `done/`                                                                     | 10m | t005                    | — **DONE** |

## Definition of done

One green `scripts/cli-compat.sh verify` run (recorded in the checklist) exercises every ✅ checklist row that makes a bex-api call, using `checkFields`-style whole-shape assertions (never single-field spot checks — the RC14 lesson), self-creating and trap-cleaning every resource it touches; the checklist states browser/device login, refresh, and logout coverage separately from the `RENDER_API_KEY` override; a deliberately broken response shape makes the relevant leg exit non-zero (mutation-proven).

## Source + Goal linkage

- **Source:** the checklist's own recorded residual (`docs/cli-compatibility-checklist.md` §Reproducing: "Not yet covered by this script … a gap worth closing so RC2/RC7–RC12 can't silently regress either"), from `w9/m2`'s t006. Proposed in `/pm-brainstorm` round 9.
- **Goal linkage:** Render parity (pillar 1) — the CLI is the fifth verified surface; unverified fixes drift (the exact rationale that built w7/m30's conformance suite).
- **Expected outcome:** a regression in any CLI-facing wire shape fails a single local command instead of resurfacing as a support-grade mystery.
- **Why now:** fifteen root causes were fixed in 48 hours across five parallel passes — the fix surface is large, fresh, and mostly unguarded; w9 owns the harness and its queue is empty.
- **Render parity closing task: omitted** — verification harness; changes no REST/GraphQL/MCP/UI surface (the `w7/m30` "the milestone IS the parity check" precedent).
