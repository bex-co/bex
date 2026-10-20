# CLI sweep 1006 — secret-file single PUT endpoint (live, 2026-10-20)

One disposable free web service created and deleted (204). A synthetic throwaway file content was used; its value is not recorded here. No new bug filed.

- `PUT /services/{id}/secret-files/{name}` (single-file upsert) → 201; the owner read returns the file; DELETE → 204. This complements the replace-set PUT from sweep 957.
- A URL-encoded traversal name (`..%2Fevil`) is decoded and refused with the same coded 400 `SECRET_FILE_INVALID` as the literal form — the validation runs after path decoding, so the encoded spelling is not a bypass.
