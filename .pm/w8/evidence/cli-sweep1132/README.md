# CLI sweep 1132 — env revision is per-service-set, not per-key (live, 2026-10-20)

One disposable free web service created and deleted (204, verified 404). No new bug filed.

- After writing keys A and B, both read back with the same revision (`evr1_…I`). Overwriting A advances the shared revision (`evr1_…M`) on both keys, including the untouched B.

The revision stamps the service's whole env-var generation, so any single-key write invalidates prior revisions for every key — consistent with a deploy being triggered from the set as a unit.
