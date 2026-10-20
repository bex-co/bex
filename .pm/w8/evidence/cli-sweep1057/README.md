# CLI sweep 1057 — service names are freed on delete (live, 2026-10-20)

Two disposable free web services created in sequence under the same name; both deleted (204s). No new bug filed.

- After deleting `qa1057-web`, recreating the same name → 201 with a fresh id and the same slug. The name-uniqueness scope (sweep 988) is over live services only; deletion releases both the name and the slug.
