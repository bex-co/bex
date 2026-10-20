# CLI sweep 896 — env-var source references are not single-PUT shapes (live, 2026-10-20)

Disposable free web service created, probed, deleted (204, then GET 404). No new bug filed.

- `fromEnvGroup` / `fromService` / `fromDatabase` bodies on the single-key env PUT → 400 `request body does not match the Render API schema` — cross-resource references are blueprint/manifest concepts (and group linking is the `/env-groups/{id}/services/{id}` verb of sweep 849); the per-key REST write accepts only the value-bearing shapes (sweep 830's oneOf), so a reference cannot be smuggled into a direct env write.
