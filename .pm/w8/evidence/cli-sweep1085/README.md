# CLI sweep 1085 — GraphQL scale honors the free-plan gate (live, 2026-10-20)

One disposable free web service created and deleted (204). No new bug filed.

- GraphQL `scaleService(numInstances:1)` → the service id; `numInstances:2` on free → `scaleService: null` with the same coded message REST gives (`the free plan is limited to 1 instance(s) and cannot scale to 2`), and the instance count stays 1. The paid gate is transport-independent.
