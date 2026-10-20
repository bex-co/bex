# CLI sweep 778 — plan enum case sensitivity (live, 2026-10-19)

Disposable free web service `qa778-web` (`srv-dbbcr22720pc73c0kn90`). Deleted and verified absent (REST 404).

- `--plan FREE` (CLI) and `{"plan":"Free"}` (REST) → 400 `plan must be one of free|starter|standard|pro|pro_plus|pro_max|pro_ultra` — enum matching is case-sensitive on both layers.
- Same-plan `{"plan":"free"}` PATCH → 200 no-op, readback unchanged.

No bug filed.
