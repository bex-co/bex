# CLI sweep 1242 — a live instance whose health check starts failing is pulled and restarted (live, 2026-10-20)

One free image web service (`qa-20261020-c1b352-live`, `srv-dbbhl6pq56vc73bn8c50`, `docker.io/library/python:3.12-alpine`, `healthCheckPath: /health`) was created over REST, then deleted (GET 404). REST was needed because the pinned CLI refuses `--start-command` for image runtimes client-side (“only supported for native runtimes”), as upstream does. No new bug filed.

- The first attempt crash-looped on a shell-precedence mistake in this sweep's own start command. Bex closed it `update_failed` with an actionable `failureReason` (“container exited shortly after start … last exit code 2 — check the service logs …”), and the logs showed the `cd` error.
- With the fixed Docker Command, `/health` disappears 150 s after each container boot. Over 12 minutes (`bex deploys create --wait`, then polling the public URL), the cycle repeated with the same shape each time:
  - 200 while healthy;
  - out of rotation (503) about 27 s after the first 404;
  - a fresh boot (new `/boot` timestamp) and 200 again about 80–105 s after the failures began.
- This matches ADR004's probes: readiness 3×10 s, then liveness after 60 s of failures, which is Render's “stop routing, then restart” contract. The events feed recorded `server_failed {reasonCode: readiness_failed}` / `server_available` pairs for each cycle.
