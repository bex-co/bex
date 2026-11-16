# rt-fix-prod-crashes ledger

Earlier runs' **Deferred** and **Skipped** findings, read at the start of every run and updated by its change (see the ground rules in [SKILL.md](SKILL.md)). One line per finding, newest first, at most 40 entries.

## Deferred

None yet.

## Skipped

- 2026-11-14 `postgres.redactLiterals` idempotency — not asserted by `FuzzRedactLiterals`: an empty literal followed by an unterminated bit-string redacts to two markers SQL re-reads as one; nothing leaks because callers redact once
