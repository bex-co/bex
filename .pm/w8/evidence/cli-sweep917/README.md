# CLI sweep 917 — project PATCH refusals correct but bare (live, 2026-10-20)

Disposable project created, probed, deleted (204, then GET 404). No new bug filed.

- PATCH `ownerId` / `id` / empty `name` on a project → 400 each, owner and name read back intact. Unlike the apps and Key Value surfaces (which name the offending field), the projects decoder answers the bare `{"error":"bad request"}` — correct refusal, weaker diagnostics. Recorded as a diagnostics-variance note in the same family as sweep 916, not filed: no write occurs and no state is spoofable.
