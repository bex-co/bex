# CLI sweep 714 — service-name length boundary (live, 2026-10-19)

Fixtures deleted and verified absent (zero `qa714` services).

- Exactly 30 characters (`qa714-` + 24 digits) → created.
- Exactly 31 characters → 400 `RESOURCE_NAME_INVALID: name must use lowercase letters, digits, and hyphens, be at most 30 characters, and not start or end with a hyphen`.

Note: an earlier probe pair in this sweep used 28/29-character names (hand-miscounted) and both were created; the exact boundary above supersedes that reading. The stated limit and the enforced limit agree.

No bug filed.
