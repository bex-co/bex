# CLI sweep 1182 — port is range- and type-checked (live, 2026-10-20)

One disposable free web service created and deleted (204, verified 404). No new bug filed.

- `serviceDetails.port` of `0`, `70000`, and the string `"8080"` → 400 each; `8080` → 200 and reads back as 8080. TCP port range (1–65535) and integer type are both enforced, with the string form refused rather than coerced (sweep 1114 discipline).
