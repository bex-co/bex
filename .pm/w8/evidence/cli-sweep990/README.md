# CLI sweep 990 — service create validation battery (live, 2026-10-20)

Refusal-side probes only; the follow-up name list proves nothing was created. No new bug filed.

- Missing name → schema 400 at `/name`; empty name → coded 400 `RESOURCE_NAME_INVALID` spelling out the rule (lowercase letters, digits, hyphens, ≤30 chars).
- Bogus `type` → schema 400 at `/type`; bogus runtime → 400 naming the unsupported value; neither repo nor image → 400 `one of repo or image is required`.
- Unknown owner workspace → 403 `FORBIDDEN` (not a create in someone else's workspace, not a 404 leak).
