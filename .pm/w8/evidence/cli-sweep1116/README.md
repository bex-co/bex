# CLI sweep 1116 — container shape mismatches refuse (live, 2026-10-20)

One disposable free web service created and deleted (204). No new bug filed.

- An object where the env-vars array belongs, an array where `serviceDetails` belongs, and a raw string body for env vars all → 400. Container shapes are schema-strict in both directions.
