# CLI sweep 591 — double suspend/resume on a `$PORT`-honoring image (sweep 590's caveat, resolved)

**Verdict: no bug filed.** Same isolated QA session, `bex-canary`, HEAD-built CLI at `4303334d2`, 2026-10-19. Owned fixture `qa591-web` (`srv-dbb96ikgnrlc73bhsrog`, free `gcr.io/google-samples/hello-app:1.0`), deleted at close: REST 404, URL 404, zero `qa591` names.

- Initial deploy went live and served `Hello, world!` within ~20s.
- **Cycle 1:** suspend → 202, URL 503; resume → 202, URL back to 200 in ~12s.
- **Cycle 2:** identical — suspend 202/503, resume 202, 200 in ~11s. Events show the clean `suspender_added` / `service_suspended` / `suspender_removed` sequence with no `server_failed`.
- This resolves sweep 590's control caveat: the readiness failure there was specific to the busybox `sh -c` fixture after its second suspend, not a platform resume defect. A known-good image suspends and resumes repeatedly and quickly.

## Unrun

- Suspend/resume of a cron job or background worker through the same cycle.
- Resume into a workspace with billing delinquency (refusal path; not safely exercisable here).
