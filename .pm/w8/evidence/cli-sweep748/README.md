# CLI sweep 748 — local launcher commands (`docs`, `code keys`) (live, 2026-10-19)

Local CLI surfaces; no agent was launched.

- `bex docs` (with `BROWSER=/usr/bin/true` in the harness) → exit 0, no output.
- `bex code keys` → the provider-key inventory (source: environment/configured/none per launcher; no key values printed).
- `bex glm --help` delegates to the underlying Claude Code help — launcher passthrough confirmed without starting a session.

No bug filed.
