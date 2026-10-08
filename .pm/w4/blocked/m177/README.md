# w4 · m177 — Report container startup failures as startup failures

**Worker:** worker4 **Goal:** a tenant whose container cannot execute its startup program can identify the missing executable from the deploy page and APIs without production Kubernetes access. **Status:** blocked

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Distinguish runtime StartError from an application crash — **DONE** | 45m | — |
| t002 | Verify both diagnosis callers and preserve adjacent failure classes — **DONE** | 35m | w4/m177/t001 |
| t003 | Render parity — **DONE** | 20m | w4/m177/t001, w4/m177/t002 |
| t004 | Simplify — **DONE** | 15m | w4/m177/t003 |
| t005 | Test coverage — **DONE** | 35m | w4/m177/t003 |
| t006 | Closeout | 10m | w4/m177/t004, w4/m177/t005 |

## Definition of done

Repeat on a new owned Free `qa-<date>-` fixture; delete it and revoke the QA session afterwards:

- Create an Existing Image web service using `traefik/whoami:v1.11.0`, port 3000, and Docker Command `/whoami -port 3000`. Open its deploy detail, then reload. While this image cannot execute `/bin/sh`, the visible diagnosis explicitly says the container could not start and names the unavailable shell. It does not claim that the application started and crashed, recommend nonexistent application output, or suggest a port bind as the cause of this known startup failure.
- Run the GraphQL request in [finding.md](finding.md), substituting the new service id. Its open deploy `stallReason` contains that same specific startup diagnosis. The helper's existing `CrashLoopBackOff` condition reason may remain; the fix must reach the current consumers rather than introducing a reason they discard.
- Cancel that first deploy, set `WHOAMI_PORT_NUMBER=3000` using Environment → Save only, then clear Docker Command in Settings and confirm Save changes. The replacement deploy reaches Live, GraphQL reports `phase: Running` and an empty `stallReason`, and `curl -sSI https://<qa-name>.onbex.co` returns 200. This healthy recovery was exercised during the hunt.

Terminal timeout behavior, REST/MCP, Events/email and genuine application-crash controls were not live-probed for this finding. They are explicit verification work in t002/t003, not claims about observed production results. Do not close until that work and the observed-state replay hold.

## Source + Goal linkage

- **Source:** user-requested infinite `/qa-find-bugs` loop with filings in w4, 2026-10-07 UTC, pass 1. Production `bex` workspace; credentials were consumed only inside the login helper from the user-specified environment file. Complete request/response and read-only pod observation are preserved in [finding.md](finding.md). Local screenshot `.playwright-mcp/qa-start-error-20261007-1.png` and probe `.playwright-mcp/qa-start-error-20261007-api.json` were checked with `ls`; the board's durable evidence does not depend on retaining those ignored files.
- **Goal linkage:** ADR008 reliable hosting and agent-operable diagnostics; ADR004 deploy health gating; ADR010 observability without `kubectl`.
- **Expected outcome:** humans and agents learn that the container cannot execute its startup program and how to correct this specific image/command combination.
- **Why now:** the newly available Docker Command flow can induce this failure on a public image, and the only actionable cause was visible through operator-only Kubernetes access. Reproduced after a fresh reload; current main still discards the relevant termination reason/message.
- **Render parity included:** the diagnosis travels through tenant-facing REST/GraphQL/MCP and dashboard deploy surfaces. Render's [image deployment](https://render.com/docs/deploying-an-image) and [Docker](https://render.com/docs/docker) docs describe command overrides, but do not establish this exact failure-message wording. No authenticated Render reproduction was performed. This milestone changes diagnostics, not command interpretation or image security policy.
- **Sizing:** 80m across implementation and shared-caller verification, plus 80m standing tasks. One reproducible finding with a shared mechanism merits a milestone rather than an inflated collection of speculative bugs.


## Progress (2026-10-08)

t001–t005 done. `stuckPodMessage` (`app_controller.go`) now checks a crash loop's last termination. When it is a `StartError`, `startErrorMessage` says the container could not start and that the program never ran, so there is no application output. It names the program when runc's `exec: "<program>": …` shape identifies it (missing, or not executable). A missing `/bin/sh` tells the user to clear the Docker Command (run as `/bin/sh -c`) or use an image with a shell. The raw OCI text is never passed through, and an unrecognized runtime message gets bounded generic startup copy. The `CrashLoopBackOff` reason is kept, so bex-api's `stallDiagnosis` admits it. Both production callers carry the message: the in-flight Ready message and terminal settlement. REST, GraphQL, MCP and the dashboard already pass it through verbatim; no consumer parses the old wording. Ordinary crashes keep the exit-code, logs and `$PORT` guidance. ImagePull, InvalidImageName, CreateContainerConfigError, probe-stall and healthy-rollout tests are unchanged and pass. ADR004 records the rule.

Tests: `TestStuckPodMessage` adds the production-captured missing-shell status verbatim (it fails on the old copy, which contained "exited shortly after start"), plus not-on-PATH, permission-denied and unrecognized-message cases. Operator `make test` and `make lint` pass.

Not verified: a Waiting `RunContainerError` before the first back-off still yields ordinary progress; the terminal timeout, Events/email and private/worker variants are pending the live replay. No backend or dashboard change was needed.
