# CLI sweep 916 — error-text consistency spot-check (live, 2026-10-20)

Disposable env group created and deleted (204). Pin re-checked (`3ede47d3c5a9`); the parallel worker's `a79c56976` has recorded loop evidence through sweep 898. No new bug filed.

- Sweep 915's KV double-prefixed refusal text (`bad request: bad request body: …`) is local to that decoder's wrapping; neighboring surfaces phrase the same class differently (env-group PATCH unknown-field dies at the validator as `invalid request body at /name`). All three refuse correctly with a 400 and no write — wording variance only, recorded here so it is not later mistaken for a behavior divergence.
