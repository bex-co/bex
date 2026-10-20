# CLI sweep 867 — `notifyOnFail` enum cycle (live, 2026-10-20)

Disposable free web services created, patched, deleted (204s; GETs 404). No new bug filed.

- The vocabulary is `default|notify|ignore` (not the `begin|end` spellings a first probe guessed): each valid value PATCHes and reads back; out-of-vocabulary values → named 400 enumerating the three. The legacy `notifyOnFail` field and the newer `notificationsToSend` coexist on the read object without disagreeing (ADR018 w3/m15 legacy-preservation contract).
