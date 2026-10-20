# CLI sweep 617 — `services instances` dialects across states and resource kinds

**Verdict: no bug filed.** Same isolated QA session, `bex-canary`, CLI at `f31004f7c`, 2026-10-19. Owned fixture `qa617-web` (`srv-dbbamn2bpdss73ci0oog`, free hello-app), deleted at close: REST 404, zero `qa617` names. The baseline Postgres id was only passed as a selector (read-only refusal path), never mutated.

- **Deploying already lists the instance:** during the first rollout the list showed the starting instance (`srv-…-rkgo…`); once live, the same id remained. By-name resolution returns the identical row.
- **Suspended → empty, not error:** after suspend, the list printed `[]`, exit 0 — an honest empty set for a service with no running instances. After resume, text mode rendered the new instance in the ID/AGE table.
- **Wrong kinds and synthetics refuse identically:** the baseline Postgres id (`dpg-…`), a synthetic Key Value id (`red-000…`), and a synthetic service id all fail with `failed to list instances: 404 Not Found`, exit 1 — cross-kind ids do not enumerate instances.

## Unrun

- Multiple instances (paid scaling) and instance turnover across a restart (575 covered the restart arm).
- Instances of a static site (no runtime instances expected).
