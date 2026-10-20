# CLI sweep 1185 — write envelopes are stricter than read shapes (live, 2026-10-20)

One disposable free web service created and deleted (204, verified 404). No new bug filed.

- Top-level PATCH of `rootDir` or `startCommand` → 400 (unknown field): fields that appear in read shapes or belong in nested envelopes (`serviceDetails`/`envSpecificDetails`) cannot be written at the top level. The write schema is its own contract, not a mirror of the read shape — the same discipline behind the port-precedence and command-envelope findings from earlier sweeps.
