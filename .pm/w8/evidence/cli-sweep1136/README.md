# CLI sweep 1136 — scale validation and the free-plan ceiling (live, 2026-10-20)

One disposable free web service created and deleted (204, verified 404). No new bug filed.

- `numInstances: 1` → 202; `0`, `-1`, and the string `"2"` → 400 each (range and type enforced; no coercion, matching sweep 1114).
- `numInstances: 2` on the free plan → 400 with a coded message naming the limit (`the free plan is limited to 1 instance(s) and cannot scale to 2`), and the instance count stays 1.

The paid scaling gate fails closed with an explanatory refusal; nothing was charged or provisioned.
