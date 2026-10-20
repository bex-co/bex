# CLI sweep 727 — deploy list cursor walk (live, 2026-10-19)

Disposable free web service `qa727-web` (`srv-dbbcc55fsg2s73akoajg`) with two deploys. Deleted and verified absent (REST 404).

`GET /services/{id}/deploys?limit=1` → newest deploy with `cursor` = its id; feeding the cursor back returned the older deploy; a third page was `[]`. No repeats, no skipped entries — keyset paging is exact on this subresource (matches the sweep-651 services walk).

No bug filed.
