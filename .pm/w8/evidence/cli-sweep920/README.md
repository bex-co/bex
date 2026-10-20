# CLI sweep 920 — repeated lifecycle verbs stay convergent (live, 2026-10-20)

Disposable free web service created, hammered lightly, deleted (204, then GET 404). No new bug filed.

- Two immediate restarts → 200/200; suspend→resume in quick succession → 202/202 with the final state `not_suspended` (last verb wins, no stuck intermediate, no 409 conflict on rapid transitions — the state machine accepts the sequence at human/API speed without a lock error).
