# CLI sweep 900 — post-closure spot-checks for 135 and 137 (live, 2026-10-20)

Read-only/refused probes; nothing created. No new bug filed.

- Closed w8/135 behaves as its acceptance promised: a `registry`-field body now → 400 `request body contains unknown field "registry"` (the handler's named refusal, replacing the old validator's `/registry` demand).
- Open w8/137 unchanged at pin `3ede47d3c5a9`: `POST /services/{synthetic}/headers` → 405 (its fix `e8d28e88e` is not an ancestor of the pin). The probe used a synthetic id, so no fixture was needed to observe the method gate.
