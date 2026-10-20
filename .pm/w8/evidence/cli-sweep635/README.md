# CLI sweep 635 — autoscaling on a free service: the plan gate fires before field validation

**Verdict: no bug filed.** Same isolated QA session, `bex-canary`, CLI at `c8717afdf`, 2026-10-19. Owned fixture `qa635-web` (`srv-dbbb38dfsg2s73ako6h0`, free hello-app), deleted at close: REST 404, zero `qa635` names. REST boundary (no CLI autoscaling verb at this pin). Re-checks the w4/365 free-plan refusal live.

- **Wire shape:** `GET /autoscaling` on the free service returns the disabled zero struct. The PUT body requires `min`/`max` and a `criteria` object with **both** `cpu` and `memory` members — wrong shapes fail with field-named body errors (`invalid request body at /criteria/memory`, `/min`), each verified by re-read.
- **Gate order:** with the full valid shape, enabling on free → `400 autoscaling requires a paid instance type` (the w4/365 behavior, live). The same refusal answers `min > max` and `percentage: 101` bodies — plan eligibility is evaluated before range validation, so no deeper validation is reachable on a free service.
- **Disable is accepted:** the all-zero disabled PUT returns 200 and reads back unchanged.
- **Synthetic service:** full-shape PUT → `404 not found` (existence precedes everything).

## Unrun

- Range validation itself (min>max, percentage bounds) — unreachable behind the free-plan gate; needs a paid fixture.
- Autoscaling criteria with both cpu and memory enabled on a paid plan.
