# CLI sweep 1187 — `autoDeployTrigger` enum and precedence (live, 2026-10-20)

One disposable free web service created and deleted (204, verified 404). No new bug filed.

- `off` → 200; `commit` → 200 and flips the companion `autoDeploy` read-back to `yes` (the trigger takes precedence over the legacy toggle, as the field contract states); a bogus value → 400 naming the full enum (`off|commit|checksPass`).
- `checksPass` on this image-backed service → 400 explaining it needs a Git repository (`a prebuilt image has no commits to check`) — the documented no-CI-gate divergence expressed as a contextual refusal, not a generic enum error.
- Observed but not filed: `commit` is accepted on an image-backed service; Render's answer for that combination is unverified against the pinned contract.
