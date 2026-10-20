# CLI sweep 991 — env-var single-key lifecycle and revisions (live, 2026-10-20)

One disposable free web service created and deleted (204). No new bug filed.

- Batch PUT then single-key PUT update the value in place; the `revision` token advances (`evr1_…E` → `evr1_…I`) on each write, so optimistic-concurrency consumers get a moving revision.
- Single-key DELETE → 204, GET → 404; a repeat DELETE → 404 (env keys are not idempotent-delete, unlike services in sweep 987 — each resource keeps its own delete contract, recorded).
