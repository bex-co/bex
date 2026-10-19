# CLI sweep 590 — group var changed while suspended: held for the next deploy, resume does not consume it

**Verdict: no bug filed.** Same isolated QA session, `bex-canary`, HEAD-built CLI at `4303334d2`, 2026-10-19. Owned fixtures `qa590-web` (`srv-dbb8u8lt4o5c73dqgg00`, free busybox serving `V=[%s]`) and `qa590-group` (`evg-dbb8u8fo17is73fus070`, `V590=before`, linked). Both deleted: service 404, group 404, zero `qa590` names.

- **Sequence observed:** service live with `V=[before]` → suspend (202, URL 503) → group var updated to `after` (200, and a fresh reveal reads `after`) → resume (202). For ~5 minutes after resume the page kept reading `V=[before]`. A manual `deploys create` then brought `V=[after]` within seconds.
- **Why this is the designed behavior, not a defect:** `rollOne` ([service.go](/lego/backend/internal/envgroups/service.go:1263)) deliberately leaves a suspended linked service pending — "held because the service was suspended when written" — and w4/m234 (`TestADeployingGroupSaveLeavesASuspendedServicePending`) pins that a suspended service is not rolled and is reported pending, exactly like an Auto-Deploy-off one. `Resume`/`setSuspended` only flips `Spec.Suspended`; it does not re-materialize config. Sweep 278 already recorded the same contract for a service's own first saved variable: suspend/resume preserves the unset state and the next standard deploy consumes the new value. The group path behaving identically is consistency, not drift.
- **Control caveat (unverified, not filed):** the follow-up control — the service's own `V590` changed while suspended, then resume — never became ready: a `server_failed / readiness_failed` event landed ~19s after resume and neither time (5+ min) nor a `restart` recovered the page within the probe window, and the container produced no app-type log lines. The fixture was deleted without isolating whether this was platform capacity, the activator path, or a real resume defect; a future sweep with a `$PORT`-honoring image should re-run the double suspend/resume cycle cleanly before treating it as signal.

## Unrun

- Clean double suspend/resume cycle on a known-good image (see caveat).
- Whether the held-change pending state is visible anywhere on the REST per-key response (the 200 carried only `{key}`).
- Group update while suspended, then group unlink before resume.
