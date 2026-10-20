# CLI sweep 1090 — service delete cascades to dependents (live, 2026-10-20)

One disposable free web service with an env var, a secret file, and a custom domain attached, then deleted (204). No new bug filed.

- After the delete, the env-vars, secret-files, and custom-domains subresources all answer 404: dependents die with the service; no orphaned rows remain addressable.
