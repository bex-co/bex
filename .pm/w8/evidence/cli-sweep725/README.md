# CLI sweep 725 — restart/suspend racing an in-flight deploy (live, 2026-10-19)

Disposable free web service `qa725-web` (`srv-dbbcaaavfpec73dm0m80`), live, then a bad-image deploy was started. Deleted and verified absent (REST 404).

- `POST restart` during the in-flight deploy → 200 (not refused); it queued its own good-image deploy.
- `POST suspend` during the deploy → 202; the bad-image deploy ended `canceled` (suspend cancels in-flight work, as in sweep 709).
- After `resume`, the restart's deploy completed `live` and the previous live deploy became `deactivated`; public URL 200 at the end.

Interleavings resolve coherently — no stuck or resurrected deploys. No bug filed.
