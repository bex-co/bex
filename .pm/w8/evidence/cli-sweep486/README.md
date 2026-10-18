# In-flight/terminal deploy dialect — sweep486

Passing functional coverage, no new issue; ship state unchanged (gates re-run at sweep end: `main` still red at `e6457568b`, run 39120931451; held commit `f62d6740e` remains local). Same isolated session as sweeps 454–485, `bex-canary`, HEAD checkout build, 2026-10-18.

State-machine dialect around non-live deploys: `services instances` during a first build is an honest `[]`; `restart` before any live deploy refuses `409 conflict: service "…" has no live deploy to restart; deploy it first`; after `live`, `deploys cancel` on the terminal deploy refuses `409 conflict: deploy "…" is already live` with the deploy row and the served HTTP 200 both unchanged. All refusals are named, coded, and side-effect-free.

Cleanup: exact-owned `srv-dbaikqbc1vqc73a22l9g` deleted by ID (404-verified); baseline deltas are concurrent foreign churn, observed read-only. No plan mutations. Session remains active; next inline sweep is 487; `f62d6740e` + evidence 477–486 await a green main.
