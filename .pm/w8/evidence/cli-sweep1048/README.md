# CLI sweep 1048 — service read shape carries no credential fields (live, 2026-10-20)

One disposable free web service created and deleted (204). No new bug filed.

- A full-text scan of the service GET response finds no credential-shaped keys (`password`, `secret`, `token`, `apikey`, `api_key`). The read model stays free of secret material by construction, complementing the show-once disciplines verified for webhooks (964) and registry credentials (968).
