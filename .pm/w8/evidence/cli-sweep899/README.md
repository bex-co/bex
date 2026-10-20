# CLI sweep 899 — plan PATCH spelling discipline (live, 2026-10-20)

Disposable free web service created, probed, deleted (204, then GET 404). No new bug filed. No paid plan value was ever sent (sweep-777 lesson).

- `serviceDetails.plan: "free"` (same value) → 200 no-op. Top-level `{"plan": …}` → 400 unknown field — plan lives only in `serviceDetails` on this PATCH.
- Bogus nested plan → named 400 enumerating the vocabulary (`free|starter|standard|pro|pro_plus|pro_max|pro_ultra`); enum matching is case-sensitive (sweep 777's `Free` refusal was the same gate).
