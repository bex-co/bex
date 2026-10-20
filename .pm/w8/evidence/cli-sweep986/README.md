# CLI sweep 986 — suspend/resume idempotency (live, 2026-10-20)

One disposable free web service created and deleted (204). No new bug filed.

- Suspend → 202, service reads `suspended: suspended` with `suspenders: [user]`; a second suspend → 202 again (idempotent, no error, no duplicate suspender).
- Resume → 202; a second resume → 202; final state `not_suspended`. Both transitions tolerate repetition without coded complaints.
