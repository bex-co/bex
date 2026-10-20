# CLI sweep 1163 — plan PATCH validation (live, 2026-10-20)

One disposable free web service created and deleted (204, verified 404). No new bug filed.

- PATCH `plan: free` (unchanged) → 200; PATCH `plan: bogus` → 400.
- PATCH `plan: starter` → 200 with the plan applied on read-back. Paid plan transitions succeed by design (they are billing operations, per the sweep 777 lesson), so this probe should have stayed on the bogus/same-free forms. The service was deleted in the same sweep and verified absent; no paid resource persists. Plan probes stay refusal-side going forward.
