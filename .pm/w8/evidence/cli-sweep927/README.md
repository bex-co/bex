# CLI sweep 927 — JSON `null` is not a delete/clear spelling (live, 2026-10-20)

Disposable free web service created, probed, deleted (204, then GET 404). No new bug filed.

- `PATCH {"name": null}` → 400 `invalid request body at /name`; `{"serviceDetails": null}` → 400 at `/serviceDetails` — explicit nulls refuse at the validator instead of being read as "clear this field" (clears use the empty-string/absent spellings verified in sweeps 869/884).
- A `?fields=` sparse-fieldset parameter → the per-operation unsupported-parameter refusal (sweep 890), so response shaping cannot be used to probe hidden fields.
