# CLI sweep 584 — `deploys create`/`cancel` dialects on an image fixture

**Verdict: no bug filed.** Same isolated QA session, `bex-canary`, HEAD-built CLI at `4303334d2`, 2026-10-19. Owned fixture `qa584-web` (`srv-dbb8g5lt4o5c73dqgeo0`, free `busybox:1.36` image), deleted at close with REST 404 and zero `qa584` names remaining.

- **Create dialects:** plain create and `--image docker.io/library/busybox:1.37` each mint a deploy carrying the requested `image.ref`. Malformed reference → `400 imageUrl: image reference is malformed: invalid reference format`. `--commit` on an image-backed service → `400 commitId is not supported for image-backed services — deploy an image tag or digest with imageUrl`. `--image` + `--commit` together fail client-side: `only one of commit or image may be specified`. `--clear-cache` on an image deploy is accepted (a no-op for image pulls; no build exists to clear).
- **Supersede semantics:** each newer deploy canceled the previously queued one (`canceled` with `finishedAt` set), leaving only the newest `update_in_progress` — the queue keeps the latest intent, matching earlier lifecycle sweeps.
- **Cancel:** canceling a real queued deploy prints `Deploy <id> successfully canceled` and the row reads `canceled`. Canceling a nonexistent or malformed deploy id → `404 (NOT_FOUND): deploy not found`; a synthetic service id → `404 not found`. No client-side format gate, same shape as the jobs boundary (sweep 556).
- **`--wait` (bounded, inconclusive):** a `--wait` create on the command-less busybox workload was still waiting after 9 minutes with the deploy `update_in_progress` (the image has no runnable command, so the workload never becomes ready). The probe was terminated by the operator; terminal exit semantics of `--wait` on a failed deploy remain unverified — see Unrun.

## Unrun

- `--wait` exit code/message on a deploy that reaches a failed terminal state, and on success against a `$PORT`-honoring image.
- `--commit` deploy on a Git-backed fixture (refused here only because the fixture is image-backed).
- Deploy cancel of an already-terminal (live/failed) deploy.
