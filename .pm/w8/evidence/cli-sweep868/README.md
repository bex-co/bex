# CLI sweep 868 — `idleTTLSeconds` lives under `serviceDetails` (live, 2026-10-20)

Disposable free web services created, patched, deleted (204s; GETs 404). Pin re-checked (still `ca30db3a3646`). No new bug filed.

- Top-level `{"idleTTLSeconds":…}` → 400 `unknown field` — the bex-extension knob is nested: `{"serviceDetails":{"idleTTLSeconds":600}}` → 200 read back `600`; `0` (the operator-resolved 15-minute default) and `30` also round-trip.
- `-10` → named 400 `idleTTLSeconds must be 0-604800`. The top-level refusal is correct strict-decoding, not a missing field: the read object emits it top-level, which makes the write spelling worth recording here since the two are asymmetric by design (`rest.go:847` folds only the nested form).
