# CLI sweep 819 — private-service/worker free-tier gates (live, 2026-10-20)

No fixture survived (both creates refused; name-filter list confirms `[]`). No new bug filed.

- `private_service --plan free` → named 400 `a private_service requires a paid plan; plan must be one of starter|standard|pro|pro_plus|pro_max|pro_ultra`.
- `background_worker --plan free` → the same named gate. Only `web_service` and `static_site` are free-tier types on bex; the CLI surfaces the server's refusal verbatim with a non-zero exit.
