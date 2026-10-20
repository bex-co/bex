# CLI sweep 857 — name validation + legacy name-as-id reads (live, 2026-10-20)

Disposable free web service created and deleted (204, then GET 404). No new bug filed.

- Uppercase name `QA857-WEB` → coded 400 `RESOURCE_NAME_INVALID` spelling out the rule (lowercase/digits/hyphens, ≤30 chars, no leading/trailing hyphen) with `params.field`/`maxLength`.
- Services still resolve by legacy name as well as id: `GET /services/hello-go` (baseline, read-only) and `GET /services/qa857-web` → 200 — the stable-id transition's documented name acceptance.
- A service from the account's other workspace resolves by id too (the caller is a member of both; membership scoping, not a leak).
