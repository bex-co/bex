# CLI sweep 880 — synthetic registry-credential bind refused pre-write (live, 2026-10-20)

Refused create only; name list confirms nothing was created. No new bug filed.

- Create an image service naming `registryCredentialId: rgc-000…` → coded 404 `validating registry pull secret for ghcr.io/example/qa880:1: not found` — the credential reference is resolved and validated before any service write, so a dangling credential id cannot create a service that only fails at first pull.
