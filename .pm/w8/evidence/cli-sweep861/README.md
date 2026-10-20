# CLI sweep 861 — malformed-body discipline on create (live, 2026-10-20)

Refused creates only; name-list verifies nothing was created. No new bug filed.

- Malformed JSON (`{not json861`) → 400 `request body does not match the Render API schema` (validator-level, no write).
- Valid JSON plus trailing garbage → 400 `request body must contain exactly one JSON value` (strict single-value decode).
- A JSON body sent as `text/plain` → 400 (content-type enforced). All three refused before any resource existed.
