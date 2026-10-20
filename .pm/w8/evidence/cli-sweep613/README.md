# CLI sweep 613 — w8/132 + w8/133 fixed on main, not yet deployed: live re-verification still shows the old behavior

**Verdict: deploy lag — notes stay open, nothing refiled.** Same isolated QA session, `bex-canary`, 2026-10-19. Checkout pulled to `f31004f7c`, QA CLI rebuilt from it, session re-verified. Read-only probes only; no fixture, no cleanup owed.

- **Code state:** `f8008fcb3` fixes w8/132 (`the task, taskRun and sandbox filters are refused by name, never ignored`) and `f31004f7c` fixes w8/133 (`a malformed blueprint generate body is a 400, not a bare 500`). Both landed on `main` after the current production pin (`1a49cb806` pins platform images to `ebfefea261db`, which predates them).
- **Live state unchanged, as expected:** `POST /blueprints/generate` with `not-json` and with an empty body still returns the bare `500 internal error`; the logs matrix still returns 100 rows for bogus `task`/`taskRun`/`sandbox` and 0 for the `host`/`instance` controls. Production is simply not running the fixes yet.
- **w8/134** has no fix commit on main; its live bare-500 behavior (sweep 596) stands.
- A concurrent worker's ship (`3086b2fbc`) committed this loop's evidence for sweeps 579–611; sweeps 612+ remain local until the next ship.

**Next check:** when the production pin moves past `f31004f7c`, re-run the sweep-595/596 probe sets. w8/132 should then answer the three filters with a named refusal on `/logs`, `/logs/values`, and `/logs/subscribe`; w8/133 should answer malformed generate bodies with a named 400. Only then should the notes be closed via `/pm`.
