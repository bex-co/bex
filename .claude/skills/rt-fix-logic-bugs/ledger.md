# rt-fix-logic-bugs ledger

Earlier runs' **Deferred** and **Skipped** findings, read at the start of every run and updated by its change (see the ground rules in [SKILL.md](SKILL.md)). One line per finding, newest first, at most 40 entries.

## Deferred

- 2026-11-13 ADR012 §7 platform-client class exemption — the ADR exempts registry-listed platform clients from the class check, `Identity.CapabilityExempt` does not, and `TestScopeClassExemptions` pins the 403; fail-closed today, needs an ADR decision rather than a bug fix

## Skipped

None yet.
