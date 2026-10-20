# CLI sweep 777 — plan PATCH applies immediately (live, 2026-10-19) — process note

Fixture `qa777-web` (`srv-dbbcqo2720pc73c0kn6g`) deleted within the same sweep and verified absent (REST 404, zero `qa777` services).

This sweep was designed as a refusal probe, but `PATCH {"serviceDetails":{"plan":"starter"}}` on the free fixture answered 200 and the readback showed `"plan":"starter"`: a plan upgrade is an immediate, accepted API write — there is no confirmation step at this layer. The disposable service was deleted seconds later, ending the brief paid-plan window; no pre-existing or other-loop resource was involved.

Lesson recorded for the loop: never aim a "probably refused" probe at a paid-plan field — paid transitions succeed by design. Future plan probes stay on the bogus-enum side only.

No bug filed.
