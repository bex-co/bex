# w6 · m5 — Live-verify workspace dashboard UX against Render + real infrastructure

**Worker:** worker6 **Goal:** close the residual verification gap `w6/m1` and `w6/m3` both shipped with: `docs/render-artifacts/workspace-lifecycle.md` (the never-produced live Render capture `m1/t001` was marked done without) exists and reflects reality, bex's dashboard delete/rename UX is checked against it, and the whole workspace lifecycle flow plus its hand-authored GraphQL definitions are verified against real infrastructure instead of an offline stub. **Status:** todo

## Tasks (in order)

| id   | title                                                                                                                              | est | depends_on |
| ---- | -------------------------------------------------------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Capture live Render workspace settings/rename/delete semantics → `docs/render-artifacts/workspace-lifecycle.md` (the never-completed `m1/t001` acceptance criteria: confirmation wording, stated resource consequences, ownership-transfer check) | 30m | —          |
| t002 | Reconcile bex's dashboard danger-zone/rename UX (`w6/m3`'s settings/delete components) against the captured semantics; fix copy/flow drift or document deliberate deviation | 30m | t001       |
| t003 | Live acceptance rerun against the prod stack (dashboard.bex.co / api.bex.co, scratch workspace): drive create → switch → rename → delete end to end in a real browser against the live Postgres + OpenFGA + Kratos, incl. the `w6/m4`-handed-off `secrets.WorkspacePurger` OpenBao-purge live-verify (re-run `m3/t005`'s DoD for real, not the offline stub; _2026-07-11: retargeted off `mock-cluster.sh`, which deploys none of this stack_) | 40m | t002       |
| t004 | Regenerate `yarn codegen` against the live bex-api schema (`VITE_API_URL` + `CODEGEN_SESSION_TOKEN` at the same bex-api t003 used — no cluster needed); diff and fix any drift in the hand-authored workspace GraphQL definitions (`dashboard/src/graphql/definitions.ts`) | 30m | t003       |
| t005 | Update `w6/m1`'s and `w6/m3`'s README follow-up notes to mark these items resolved                                                       | 15m | t004       |
| t006 | Render parity: workspace lifecycle UX vs Render, final check — add the missing Workspaces/owners row to `docs/render-parity.md`          | 20m | t005       |
| t007 | Simplify: workspace UX verification changes                                                                                              | 20m | t006       |
| t008 | Test coverage for reconciled UX                                                                                                          | 25m | t006       |
| t009 | Closeout: verify DoD, mark done, move to `done/`                                                                                         | 15m | t008       |

## Definition of done

`docs/render-artifacts/workspace-lifecycle.md` exists with verbatim-captured Render delete/rename semantics; bex's danger-zone/rename UX matches it or documents deliberate drift; the full create → switch → rename → delete lifecycle flow is verified live against real Postgres + OpenFGA + Kratos, not a stub; the dashboard's hand-authored workspace GraphQL types are confirmed against a live-regenerated schema.

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w6` ("more for w6") 2026-07-09, tracing `w6/m1/t001`'s unmet acceptance criteria (its task file requires producing `docs/render-artifacts/workspace-lifecycle.md`; confirmed absent via `ls docs/render-artifacts/`, despite the task being marked done) and `w6/m3`'s own "Follow-ups" section (live-cluster rerun + codegen diff, both explicitly flagged as not done, verified against a stub only).
- **Goal linkage:** Render-parity for the workspace lifecycle dashboard surface (`docs/vision.md` pillar 1); closes residual risk `w6/m3` itself flagged as "narrow but real."
- **Expected outcome:** bex's workspace delete/rename UX becomes a verified clone of Render's rather than a best-guess design; the dashboard's workspace GraphQL layer is proven against a live schema instead of a hand-authored guess.
- **Why now:** both gaps are small and already precisely identified — cheap to close now, before `w4/m12` (members) and other `w5` follow-ups build further UI on top of an unverified foundation.
- **Render parity: included** (standing task) — this milestone's entire purpose is Render-UX verification.
