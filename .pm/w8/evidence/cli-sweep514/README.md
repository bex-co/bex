# CLI sweep 514 — service-detail required-field completeness (read-only)

**Verdict: no bug filed.** The sweep 512/513 method applied to a service: raw `GET /v1/services/{id}` on a pre-existing baseline web service (read-only; never mutated) diffed against the top-level `required` set of the vendored Render response schema. Same isolated session, HEAD build `c603c8ec2`, 2026-10-19T09:16Z.

| check | observed result |
| --- | --- |
| Top-level required fields | None missing, none `null` across 28 payload keys — including `ownerId`, `dashboardUrl`, `updatedAt`, `rootDir`, and `suspenders`, all of which the conformance fixture's minimal view omits (allowlisted there, present live). |
| `serviceDetails` | Carries the expected web-service fields (`env`, `runtime`, `plan`, `region`, `url`, `internalAddress`, `healthCheckPath`, `numInstances`, …). Its union-shape divergence is the already-allowlisted, ADR018-documented partial projection — not re-litigated here. |

## Safety and unrun checks

- Read-only against a pre-existing resource (the hunt rules permit safe metadata reads); nothing created or mutated, no secret-bearing fields printed.
- Scope note: the required-field method across the three resource kinds is now exhausted — services and Key Value are complete live, Postgres had the one real gap (w8/129). Later sweeps should rotate to behavioral surfaces again.
- Unrun: per-type `serviceDetails` required-field diffs for cron/private/worker/static (the union divergence makes a mechanical diff low-signal; type-specific readbacks were covered behaviorally by sweeps 454–500), list-item completeness.
- Deploy lag means deployed revisions can precede local `main`.
