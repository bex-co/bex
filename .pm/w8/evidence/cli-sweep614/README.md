# CLI sweep 614 — first deploy with an impossible health check: long stall, no terminal verdict in 13 min, instant recovery on fix

**Verdict: no bug filed.** Same isolated QA session, `bex-canary`, CLI at `f31004f7c`, 2026-10-19. Owned fixture `qa614-web` (`srv-dbbac34gnrlc73bhst2g`, free busybox httpd, created with `--health-check-path /qa614-nope`), deleted at close: REST 404, zero `qa614` names. Completes sweep 612's Unrun item.

- **The stall:** with no prior release, the deploy sat in `update_in_progress` and the service in phase `Deploying` with the URL at 503 for the entire 13-minute observation window. No terminal failure was reached in that window; the deploy's eventual failure deadline, if any, is longer than 13 minutes (not probed to exhaustion — see Unrun).
- **It really was the health gate:** patching `healthCheckPath` back to `/` queued a new deploy that went `live` in under 20 seconds and served the app (`qa614`). The workload had been healthy the whole time; only the check path was unsatisfiable.
- **Diagnosis wart (recorded, not filed):** the stalled deploy's `deploy_started` event carried a `stallReason` about a crash-looping container (`last exit code 0`) — that described the superseded command-less first pod, not the current httpd pod, whose only problem was the 404 check path. Event-stamped diagnoses can lag the revision they attach to; one observation, semantics not isolated enough to file.
- Contrast with sweep 612 (established service): there the same stall kept the old release serving at 200 throughout. Here, with nothing live yet, the honest 503 "first deploy in progress" page is the correct outward state.

## Unrun

- The actual terminal deadline for a health-gated first deploy (would need a 30+ minute single-fixture watch).
- Health-check 3xx/500 responses (only 404 probed).
