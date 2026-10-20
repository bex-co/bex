# CLI sweep 1015 — GraphQL env-var mutation shares the REST store (live, 2026-10-20)

Two disposable free web services created and deleted (204s). No new bug filed.

- GraphQL `setEnvVar` returns `Boolean` (a sub-selection guess gets the precise `must not have a sub selection` error). The correct form → `true`, and the REST single-key GET immediately reads the value back with a fresh revision — one env store across transports, matching the notification-store result in sweep 952.
