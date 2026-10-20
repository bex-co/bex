# CLI sweep 968 — registry credentials full cycle, secret never echoed (live, 2026-10-20)

One disposable registry credential created with a fake token, exercised, deleted (204; GET 404; no `qa968` leftover in the list). No new bug filed.

- First-pass 400 was a harness shape error: the bex body is `{name, host, username, authToken}` (the Render `registry`/`password` field names are refused by name, per the closed [w8/135](../done/135.md) acceptance).
- Create → 201; neither the create response, the GET, nor a PATCH secret-rotation response ever echoes the token value (checked by substring, never printed). Responses carry only `host, id, name, ownerId, status, username` and timestamps.
- Rename PATCH → 200; rotation PATCH → 200; DELETE → 204.
