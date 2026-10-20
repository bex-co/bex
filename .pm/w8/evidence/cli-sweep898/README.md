# CLI sweep 898 — w8/135 live-verified at pin `3ede47d3c5a9` and closed (live, 2026-10-20)

Acceptance re-run for [w8/135](../../done/135.md) after the 03:15Z pin; the finding is now in `done/`.

- First retests (within ~5 min of the pin) still returned the old `400 invalid request body at /registry` — rollout in flight. On retest at 03:23Z the full acceptance passed:
  - `POST /registrycredentials` with the bex body (`name`/`host`/`username`/`authToken`) → 201, secret never echoed (the credential landed in the caller's default workspace, the documented resolution recorded in sweep 837).
  - `PATCH {"name":"qa135-renamed"}` → 200.
  - The Render `registry`-field body → 400 naming `/registry`.
  - DELETE → 204, GET → 404 (fixture removed; nothing left behind).
- w8/137 and w8/138 fixes are **not** in this pin (`e8d28e88e`/`d98da2870` not ancestors of `3ede47d3c5a9`); both stay open for the next pin.
