# CLI sweep 1013 — pull-request previews field absence (live, 2026-10-20)

One disposable free web service created and deleted (204). No new bug filed.

- The service wire carries no previews field, and PATCHing a `previews` object → 400 unknown field. PR previews are a documented non-goal in the compatibility ledger, so the absence is the designed answer.
