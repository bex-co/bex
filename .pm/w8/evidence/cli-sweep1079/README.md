# CLI sweep 1079 — SSH keys surface, refusal side (live, 2026-10-20)

One disposable free web service created and deleted (204). No SSH key was added (bogus material only). No new bug filed.

- `GET /ssh-keys` → 200 for the account. Adding a malformed public key → coded 400 `invalid SSH public key`. No real key material was generated or uploaded by this loop.
