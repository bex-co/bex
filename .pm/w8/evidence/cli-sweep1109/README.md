# CLI sweep 1109 — secret-file name charset via the single PUT (live, 2026-10-20)

Two disposable free web services created; the first probe's harness crash (a raw space in the URL) left one fixture, found and deleted before the retry; the second was deleted normally (204; GET 404). No new bug filed.

- URL-encoded space in the file name → decoded, then refused with the same coded 400 `SECRET_FILE_INVALID` (validation after decoding, as in sweep 1006). A dotfile name (`.hidden1109`) → 201: leading dot is inside the accepted `[-._a-zA-Z0-9]+` class.
