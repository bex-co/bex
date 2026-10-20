# CLI sweep 636 — manual scaling lives at `POST /scale`, not in the service PATCH

**Verdict: no bug filed.** Same isolated QA session, `bex-canary`, CLI at `c8717afdf`, 2026-10-19. Owned fixture `qa636-web` (`srv-dbbb3rlfsg2s73ako6jg`, free hello-app), deleted at close: REST 404, zero `qa636` names. REST boundary (the pinned CLI's `services update` refuses `--num-instances` client-side: "use the dashboard").

- **Not a PATCH field:** `serviceDetails.numInstances` in a service PATCH → `400 request body contains unknown field "numInstances"` for every value tried (3, 0, -1, 101, 2.5, "2"); a top-level `replicas` is likewise unknown. The read model's `numInstances`/`replicas` are outputs, not inputs, on this route.
- **The scale verb:** `POST /v1/services/{id}/scale {numInstances}` (202). Wrong key (`replicas`) → `400 invalid request body at /numInstances`.
- **Validation order and caps:** out-of-range counts (0, -1, 101) → `400 numInstances must be 1-100` — range first. In-range on the free plan (`3`) → `400 the free plan is limited to 1 instance(s) and cannot scale to 3`. Scaling to the current count (`1`) → `202` no-op. Synthetic service → `404`.

## Unrun

- Scaling a paid service up and watching both instances serve (paid — never submitted).
- Scale-down from >1 (same gate).
