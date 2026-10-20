# CLI sweep 1167 — batch env PUT is replace-all, last-wins within a batch (live, 2026-10-20)

One disposable free web service created and deleted (204, verified 404). No new bug filed.

- Batch PUT of keys A+B → 200. A second batch containing key A twice → 200 and reads back only A with the second value: the batch replaces the whole set (B is gone) and duplicates within the batch resolve last-wins — both behaviors already recorded for batch writes; re-verified here at the current pin.
- Batch PUT of `[]` → 200 and the set reads empty: replace-all with an empty list is a valid clear, not an error.
