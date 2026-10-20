# CLI sweep 967 — blueprint id gate: unknown ids 404, malformed ids 400 (live, 2026-10-20)

Read-only probes on the baseline blueprint; baseline never mutated (no sync attempted). No new bug filed.

- First pass flagged a suspected bug — a "well-formed" unknown blueprint id answered 400 `invalid path parameter "blueprintId"`. Verification against the handler source dissolved it: the pinned pattern is `^(?:blp|exs)-[0-9a-z]{20}$` (`render_openapi.go`, `renderBlueprintIDPattern`, widened bex-side as a strict superset per its design comment), and the probe id actually carried 22 characters. Correctly rejected as malformed.
- Retest with true 20-char unknown ids (both `blp-` and Render's `exs-` prefixes) → coded 404 `blueprint not found`. The gate refuses only syntax; existence is the store's honest 404, exactly as the compatibility comment promises.
- Blueprint list/get on the baseline → 200; `POST /blueprints/validate` without a repo → 400.
