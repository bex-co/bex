# CLI sweep 773 — positional-argument posture (live, 2026-10-19)

Read-only.

- `bex services <service-id>` (no get subcommand exists) → the full list, positionals ignored.
- `bex whoami extra-arg` → normal whoami output, argument ignored.

Same pinned-cobra posture classified in sweeps 598/695 — recorded for completeness, not a finding.

No bug filed.
