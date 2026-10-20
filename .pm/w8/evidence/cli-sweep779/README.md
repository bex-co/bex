# CLI sweep 779 — deleting a linked env group (live, 2026-10-19)

Fixtures `qa779-web` + linked `qa779-group` both deleted and verified absent (service 404, group list `[]`).

Deleting the group while a service was linked → 204, group gone; the service survived with its own env-var list untouched (`[]` — group values were never copied into it). Detach-on-delete, consistent with the sweep-668 environment teardown philosophy.

No bug filed.
