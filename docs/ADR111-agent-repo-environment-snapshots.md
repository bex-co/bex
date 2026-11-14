# ADR111 — Repository environment snapshots: sessions boot from a build of the setup steps (phase 2)

**Status:** Accepted, 2026-11-14. Implemented by w9/m407. Closes [ADR109](ADR109-agent-repo-environment.md) D2's divergence and amends its D3.

**Related decisions:**

- [ADR109](ADR109-agent-repo-environment.md): repository environments, phase 1 (setup steps run at every sandbox's start).
- [ADR104](ADR104-sandbox-render-parity.md) D4 (the privileged commit path), D11 (the workspace sandbox storage cap) and § Images (imported images and where they live).
- [ADR042](ADR042-sandbox-cluster-substrate.md) D5: pause commits a sandbox's root filesystem to an OCI image.
- [ADR098](ADR098-agent-session-secrets.md) D9: which secrets a session receives unnamed.
- [ADR059](ADR059-agent-sandbox-hibernation.md) (superseded): every turn runs in a fresh sandbox.

## Context

Devin builds each environment into a **snapshot** and boots every session from it ([Blueprints § Builds and sessions](https://docs.devin.ai/onboard-devin/environment/blueprints), [Blueprint reference](https://docs.devin.ai/onboard-devin/environment/blueprint-reference), checked 2026-11-14):

- Builds have the statuses queued, building, success, partial, failed and cancelled. "By default, Devin uses the latest successful build's snapshot."
- A build runs when a blueprint is saved, a repository is added or removed, a repository secret is added, someone clicks "Build snapshot", and "roughly every 24 hours". "Only one build runs at a time"; new triggers "cancel any queued build and start fresh".
- "Individual steps time out after 3 hours."
- "Secrets are injected before every step runs during builds" and "are scrubbed from the snapshot image itself", but files a step writes "will be baked into the snapshot".
- At session start "the latest code is pulled for the relevant repositories"; `maintenance` commands are shown to the agent and not run.

bex phase 1 (ADR109 D2) runs the setup steps in every sandbox, so every turn waits for them. Every piece a build needs already exists:

- Agent sandboxes run on the user-sandbox substrate (`internal/sandbox/service.go` `CreateAgentSessionSandbox`), and the repository checkout lives in the container's root filesystem (`/workspace`; the BatchSandbox template mounts no volume).
- Pause commits a sandbox's root filesystem to `snapshots/<ws>-sandbox/<ref>-sandbox:snap-gen<N>` through the OpenSandbox controller's commit Job (ADR042 D5).
- Imported images live at `snapshots/<ws>-sandbox/images/<id>`, and a sandbox create can swap its template image for one (ADR104 § Images).

## Decision

### D1 — A build bakes one repository's setup into an image

A build is an agent sandbox that runs only the checkout and the setup steps, then becomes an image:

1. bex-api starts an agent sandbox from the `agent` template in the driver's **build mode** (`BEX_AGENT_MODE=build`). The sandbox gets the environment (`BEX_REPO_ENVIRONMENT`), the repository (`BEX_AGENT_REPOSITORY`), the build's secrets (D6), and a git proxy URL bound to the build. Through that URL the gateway serves only a clone or fetch of that repository (D11).
2. The driver clones the repository at the default branch's head into `/workspace` and runs the setup steps in order, as phase 1 does, with output redacted (ADR098 D6). Before it writes anything final, it scrubs its own per-run state: Git credential and auth-header config, the build's proxy as `origin` (replaced by the repository's public GitHub URL), and the driver's own logs. It then writes a status file naming each step's exit, duration and redacted output tail, and waits. The status file is absent until the build ends.
3. bex-api reads the status and deletes the status file (a failed delete fails the build), so no session booted from the image can read a build's verdict as its own. It then pauses the sandbox, which commits its root filesystem. It copies the committed image to `snapshots/<ws>-sandbox/images/<build-id>:image`, records its digest and size, and terminates the builder. The storage meter skips agent sandboxes, so terminating a builder deletes its pause images explicitly.

Only setup steps make a build. An environment with checks and no setup steps has nothing to bake, so it never builds; its sessions run as phase 1, which runs no setup.

### D2 — Statuses and failure codes

A build is `queued`, `building`, `success`, `failed` or `cancelled`. These are Devin's statuses without `partial`: a bex build covers one repository, so it either produced a usable image or it did not.

A failed build carries a `failureCode` and a reason:

- `setup_failed`: a step exited non-zero; the reason names the step and its exit code.
- `step_timeout` and `build_timeout`: a step, or the whole build, passed its limit (D5).
- `checkout_failed`: the clone of the default branch failed.
- `storage_limit_reached`: the image would pass the workspace's storage cap (D9).
- `internal_error`: bex's own side failed (the sandbox, the commit, the copy, or the driver's own scrub). A build that fails this way is retried after 15 minutes.

`cancelled` marks a queued build that a newer trigger replaced, and a queued or running build whose environment was deleted.

### D3 — Triggers and concurrency

A build is queued for a repository whose environment has setup steps when:

- its environment is saved and the setup steps changed (D7);
- a repo-scoped secret for that repository is set or changed (Devin rebuilds when "a repository secret" is added). Workspace-scoped secrets do not trigger builds, as Devin's organization secrets do not;
- an admin asks for one ("Build now" in the dashboard, the API or MCP);
- its newest successful build is about 24 hours old (the daily refresh; the scheduler adds jitter so a workspace's refreshes spread out). The refresh also builds a repository that never built, or whose newest build is of an older version because the save's own trigger was refused, and one whose newest success ran an older agent image (D4).

Saving an environment never fails because of a build: a refused or failed trigger is logged, and the refresh picks the repository up.

At most one build per repository runs at a time. A newer trigger replaces a queued build (the old one becomes `cancelled`) and never stops a running one. A workspace runs at most two builds at once, and builds share the workspace's concurrent agent sandbox limit with sessions: a build waits in `queued` while sessions hold the slots.

At most 12 builds per repository start in any 24 hours. A trigger past that is refused with `AGENT_REPO_ENVIRONMENT_BUILD_RATE_LIMITED` (429), whose `nextBuildAt` says when the next one may start. The daily refresh never hits the limit, since it runs once a day. Build now on an environment with no setup steps is refused with `AGENT_REPO_ENVIRONMENT_NOTHING_TO_BUILD`.

### D4 — Sessions boot from the newest matching build

At dispatch, bex-api resolves the newest `success` build whose version equals the environment's current version (D7), whose image still exists, and whose builder ran the session template's current agent image. A build therefore never boots a session into an older driver or toolchain than the template's; the daily refresh rebuilds after an agent image change. The session's sandbox boots from that image, and the driver runs in **prebuilt mode**:

- The driver fetches and checks out the session's branch over the existing checkout, as `ensureRepo` already does for a reused sandbox. The session therefore starts from the latest code, as Devin's do.
- It skips the setup steps. The prompt overlay lists them as already run in the snapshot, so the agent can rerun one when the dependencies changed (Devin shows `maintenance` and does not run it).
- The overlay still lists the checks (ADR109 D6).

A session reads the build it booted from as `environmentBuildId`.

With no matching build, the session runs as phase 1, exactly as before. That covers a repository that never built, a build still running, a failed newest build, an environment changed since its last success, and an image deleted. A failed build therefore never blocks sessions, and a broken setup step still fails a phase 1 turn with `AGENT_SESSION_SETUP_FAILED` (ADR109 D5).

### D5 — Time limits

Each build step has Devin's 3-hour limit, which the driver enforces (`step_timeout`), and the whole build has 3 hours, which bex-api enforces by terminating the builder (`build_timeout`). A build runs off any turn's clock, so ADR109 D3's per-turn limits (10 minutes per step, 20 for all) stay only for phase 1's setup at a turn's start.

### D6 — Secrets

A build receives the secrets a session on that repository receives unnamed (ADR098 D9): workspace-scoped secrets and the repository's repo-scoped secrets. It never receives explicit-scope secrets, since no session named them.

- Environment variables are not part of the image: a commit keeps files, not processes or their environment, so the image is free of the injected values, as Devin's are.
- A file a step writes stays in the image, as Devin warns for its snapshots. The dashboard says so beside "Build now".
- Every session on the repository receives the same secrets unnamed, so an image cannot hand a session a value it would not have received anyway.

### D7 — The environment's version

An environment's version is the SHA-256 of its setup steps' canonical JSON, stored on every write. Checks are not part of the version: they change the overlay, not the image. A session uses a build only when the versions match, so editing a step switches sessions back to phase 1 until the new build succeeds.

### D8 — Retention

A repository keeps the images of its newest successful build and the successful build before it. The older image is the fallback while a newer build is verified. Older images are deleted when a newer build succeeds. A failed or cancelled build leaves no image.

Build records are kept for the repository's last 20 builds, so the dashboard and the API can show history and logs. Retention runs after every build, failed ones included.

Deleting an environment cancels its queued and running builds and deletes its images. Deleting a workspace deletes them with its other sandbox images (ADR104 § Images, Cleanup).

### D9 — Storage and charges

A build's image is at most 10 GB compressed, the imported-image limit. It counts toward the workspace's sandbox storage cap (ADR104 D11) as imported images do, and a build that would pass the cap fails with `storage_limit_reached`. Images are not charged, as imported images are not (ADR104 § Images: Render documents no charge for them, and Devin documents none for snapshots). The builder sandbox's compute counts like an agent session's sandbox.

### D10 — Surfaces

Admins (`can_manage`) start builds; members who can start sessions (`can_create`) read them, as they read environments (ADR109 D7). Build now is audited under the target `agent-repo-environment-build`.

- REST: `GET /v1/agent-repo-environments/{owner}/{repo}/builds` (`{ownerId, repo, builds}`), `POST /v1/agent-repo-environments/{owner}/{repo}/builds` (Build now, 202 with the queued build), and `GET /v1/agent-repo-environment-builds/{buildId}`, whose body includes the steps with their redacted output.
- GraphQL: `agentRepoEnvironmentBuilds(ownerId, repo)` (steps omitted in lists), `agentRepoEnvironmentBuild(ownerId, id)`, `buildAgentRepoEnvironment(ownerId, repo)`, and `version` and `latestBuild` on an environment. `sizeBytes` and step `durationMs` are Float, since GraphQL's Int overflows at 2 GB.
- MCP: `list_agent_repo_environment_builds`, `get_agent_repo_environment_build` and `build_agent_repo_environment`.
- Dashboard: each repository on Agent settings → Repository environments shows its newest build's status and time, its log, and "Build now".
- Sessions: `environmentBuildId` on every session read.

### D11 — Security review of the build path

Builds add two triggers, an environment save and a daily schedule, to the commit Job that the user Suspend verb already reaches. That Job mounts the node's `containerd.sock` (node-root equivalent, ADR104 D4). ADR104 D4's constraints, applied to builds:

- **A build principal can only clone.** The builder is labelled with its build id where a session's id would be, and with a `bex-agent/build-<id>` branch that is never pushed. The git proxy refuses every receive-pack and pull-request request from a build id before admitting it. The credential mint accepts a build only while it is `building`, only from its builder Pod (name and UID), and only for its own repository and build branch. It then mints a GitHub token limited to that one repository with `contents` and `metadata` read. The egress grant admits the running builder, so a workspace's network tier applies to setup as in phase 1.
- **Nothing caller-chosen reaches the Job.** bex-api calls OpenSandbox's pause on a sandbox id it created; the controller derives the Job's image, mounts and arguments from its own flags and the sandbox's pod. A build request carries a repository name and nothing else.
- **Rate limits:** one running build per repository, two per workspace, and 12 starts per repository in 24 hours (D3). Builds and sessions share the workspace's live agent sandbox count, so builds cannot add sandboxes beyond a plan's limit.
- **Size and time:** a 3-hour build limit (D5), the commit Job's existing timeout, the 10 GB image cap, and the workspace storage cap (D9).
- **No privileged Pod left behind:** a build that fails after its pause still terminates its builder. The controller's commit Job carries its own TTL and is confined to `opensandbox-snapshot` (ADR042 D5). The build sweeper fails a build whose worker stopped heartbeating, or one past 3 h 45 m, and terminates its builder. A build's image is deleted from the registry no sooner than 24 hours after retention or an environment delete marks it, so a session that already resolved it, or a sandbox rescheduled onto another node, can still pull it. Builds are claimed under a row lock, so two bex-api replicas never run the same build.
- **Untrusted code in the builder:** setup steps are tenant code running in the tenant's own gVisor sandbox with the session's egress tier, as in phase 1. The image they produce boots only that workspace's sessions, from that workspace's registry namespace, with the namespace's read-only pull secret (ADR104 § Images).

Residual risk: a build makes the commit Job run more often, and a compromised controller remains node-root, as for every pause (ADR042, ADR104 D4). The security review lineage tracks it (docs/AGENTS.md).

## Consequences

- A repository with setup steps starts each session from an image with its dependencies installed: no setup wait once a build succeeded.
- ADR109 D2's divergence from Devin is closed. Phase 1 remains the fallback whenever no matching build exists.
- Builds use sandbox compute and registry storage that phase 1 did not; the storage cap and rate limits bound both.

## Not adopted

- Pinning a build (Devin's pin) and multi-repository builds.
- Organization-wide blueprints and `post-build` (ADR109 Not adopted).
- Building environments that have only checks.
