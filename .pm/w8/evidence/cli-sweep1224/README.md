# CLI sweep 1224 — blueprint preview degrades gracefully; validate is schema-strict (live, 2026-10-20)

No blueprint was created. No new bug filed.

- `POST /blueprints/preview` for an inaccessible repo → 200 `{found: false, reason: "repo_not_found_or_no_access"}` with a human-readable error: preview reports the fetch outcome as data, not as an HTTP failure.
- `POST /blueprints/validate` with JSON guesses (`yaml` field, repo fields) → 400 `request body does not match the Render API schema`: the validate verb takes its declared manifest envelope (multipart per Render's contract) and schema-refuses anything else before parsing.
- The blueprint list reads 200 with the baseline blueprint present (read-only).
