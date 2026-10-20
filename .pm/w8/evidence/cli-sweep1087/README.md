# CLI sweep 1087 — service name length boundary (live, 2026-10-20)

Three disposable free web services created and deleted (204s). No new bug filed.

- A 30-character name → 201 (the documented maximum, sweep 990); 31 characters → coded 400 `RESOURCE_NAME_INVALID` restating the rule. The boundary is exact.
