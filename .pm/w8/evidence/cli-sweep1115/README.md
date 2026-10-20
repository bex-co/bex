# CLI sweep 1115 — autoDeploy accepts only Render's yes/no strings (live, 2026-10-20)

One disposable free web service created and deleted (204). No new bug filed.

- `{"autoDeploy":"yes"}` → 200 (reads `autoDeploy: yes`); boolean `true`, string `"true"`, and number `1` → 400 each, with the stored value unchanged after every refusal. The field speaks Render's exact `yes|no` vocabulary with no boolean coercion — the mirror of sweep 1114's numeric strictness.
