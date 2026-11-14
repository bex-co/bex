# ADR109 — Repository environments for cloud agent sessions: setup steps and checks (phase 1)

**Status:** Accepted, 2026-11-14. Implemented by w9/m387. Amends [ADR047](ADR047-cloud-coding-agent-sessions.md) D5. D2 and D3 amended by [ADR111](ADR111-agent-repo-environment-snapshots.md) (phase 2, w9/m407).

**Related decisions:**

- [ADR047](ADR047-cloud-coding-agent-sessions.md) D5: the workspace agent environment (network tier, custom hosts, model).
- [ADR059](ADR059-agent-sandbox-hibernation.md): every turn runs in a fresh sandbox.
- [ADR090](ADR090-agent-owned-delivery.md): the prompt overlay and agent-owned delivery.
- [ADR098](ADR098-agent-session-secrets.md) D9: workspace- and repo-scoped secrets.
- [ADR099](ADR099-agent-skills.md) D5: delivery at dispatch.

## Context

Devin configures each repository's environment with a **blueprint** ([Environment](https://docs.devin.ai/onboard-devin/environment), [Blueprint reference](https://docs.devin.ai/onboard-devin/environment/blueprint-reference), checked 2026-11-13):

- `initialize` installs tools and runtimes; `maintenance` installs the project's dependencies. Both run when a **snapshot** is built: when the blueprint changes and about every 24 hours. Every session boots from that snapshot; "at the start of every session, `maintenance` commands are not auto-executed".
- `knowledge` holds named notes that are never executed. By convention `lint`, `test` and `build` say how to check the work, and Devin uses them to verify what it delivers.
- Repository-level commands run from the repository root. Each step has a 3-hour limit. Repository secrets are available to blueprint steps.

bex had none of this. The workspace agent environment carries only the network tier, custom hosts and model (`lego/backend/internal/agentsessions/environment.go`). Every turn runs in a fresh sandbox (ADR059), so an agent rediscovers how to install and check the repository on every turn, paying model tokens for it, and nothing tells it which checks the project expects.

## Decision

### D1 — A repository environment: setup steps and checks

A workspace may declare one environment per repository (`owner/name`, normalized like a session's repository):

```json
{
  "repo": "acme/api",
  "setup": [{ "name": "deps", "run": "npm ci" }],
  "checks": [
    { "name": "lint", "run": "npm run lint" },
    { "name": "test", "run": "npm test" }
  ]
}
```

- `setup` — ordered shell steps (Devin's `initialize` + `maintenance`), run from the repository root.
- `checks` — named commands (Devin's `knowledge` entries `lint`, `test`, `build`, plus custom names) the agent runs before it delivers. They are never run by bex itself.

### D2 — Phase 1 runs setup at the start of every sandbox

The driver runs the setup steps after the checkout (`ensureRepo`) and before the agent's turn, once per sandbox. Since every turn is a fresh sandbox (ADR059), that is once per turn.

**Phase 2 closes the divergence ([ADR111](ADR111-agent-repo-environment-snapshots.md), w9/m407; until it ships, phase 1 diverges).** Devin runs setup when it builds a snapshot and boots every session from it. Phase 1 runs it at each turn's start instead, so each turn pays the setup's time. ADR111 builds an image from the setup steps and boots sessions from the newest matching build. This per-sandbox setup now runs only when no matching build exists: before the first build succeeds, after the steps change, or when the newest build failed.

### D3 — Limits

| Limit | Value |
| --- | --- |
| Repositories with an environment per workspace | 50 |
| Setup steps | 10, each `run` at most 8 KiB |
| Checks | 10, each `run` at most 2 KiB |
| All commands of one environment, encoded | 64 KiB (it reaches the sandbox as one env var, which Linux caps at 128 KiB) |
| Step and check names | `[A-Za-z0-9][A-Za-z0-9 ._-]{0,63}`, unique within their list |
| Time per setup step | 10 minutes |
| Time for all setup steps | 20 minutes |

The per-turn limits are far below Devin's 3-hour build-time limit because phase 1 runs on the turn's clock (D2). A step past its limit is killed and fails the turn (D5). Builds (ADR111 D5) have Devin's 3 hours per step and per build; these per-turn limits apply only to phase 1's setup at a turn's start.

### D4 — Environment and secrets

Setup steps run with the agent's own process environment (`PATH`, `HOME` and the rest of the sandbox's env, plus the agent profile's variables): the session's applied secrets (ADR098 D9 — workspace-scoped, repo-scoped and named) are present, as Devin's repository secrets are to blueprint steps. Their output passes through the driver's redactor (ADR098 D6) before it reaches the transcript, so no secret value is published. Network egress is the session's (ADR047 D5): the package registries are open in every tier.

### D5 — Visibility and failure

- Each step appears in the transcript as a tool call named `setup: <name>` (input: the step name and command; output: the exit, the duration and a redacted output tail — 2 KiB for a step that succeeded, since setup runs every turn, 8 KiB for one that failed), before the agent's first message. No new transcript part type is needed.
- A step that exits non-zero or passes its time limit ends the turn: the agent does not start, no model spend accrues, and the session fails with `failureReasonCode` **`AGENT_SESSION_SETUP_FAILED`** and a reason naming the step and its exit code (or the time limit). The Completer accepts this one code from the untrusted driver's status file; any other code it reports is ignored.
- A steer after the environment is fixed runs setup again and re-delivers the task (ADR047 continuity: setup-phase failures re-deliver the task).

### D6 — The overlay names the checks

When the repository has checks, the prompt overlay (ADR090) lists them and asks the agent to run each from the repository root before it commits, and to name each result in the pull request body or its final message. A failing check does not block delivery; it is reported.

### D7 — Storage and surfaces

- Postgres `agent_repo_environments (workspace_id, repo, setup jsonb, checks jsonb, updated_at)`.
- Admins (`can_manage`) write; members who can start sessions (`can_create`) read, since the environment shapes their sessions.
- REST: `GET /v1/agent-repo-environments`, `PUT|DELETE /v1/agent-repo-environments/{owner}/{repo}`. GraphQL: `agentRepoEnvironments`, `setAgentRepoEnvironment`, `deleteAgentRepoEnvironment`. MCP: `list_agent_repo_environments`, `set_agent_repo_environment`, `delete_agent_repo_environment`. Refusals: `AGENT_REPO_ENVIRONMENT_INVALID` (shape or limits), `AGENT_REPO_ENVIRONMENT_LIMIT_EXCEEDED` (repositories), `AGENT_REPO_ENVIRONMENT_NOT_FOUND`.
- Dispatch passes the session repository's environment to the driver as `BEX_REPO_ENVIRONMENT` (JSON); a session with no repository, or a repository with no environment, gets none and runs exactly as before.
- Dashboard: Agent settings → Repository environments.

## Consequences

- An agent starts with its dependencies installed and knows the project's own checks, instead of rediscovering both each turn.
- Each turn of a repository with setup steps takes longer to start until phase 2 snapshots land.
- A broken setup step fails sessions quickly and visibly, with the step named, instead of an agent improvising around a missing dependency.

## Not adopted (phase 1)

- Snapshots built from the setup: phase 2, [ADR111](ADR111-agent-repo-environment-snapshots.md).
- An organization-wide blueprint (Devin's org-level steps) and `post-build`.
- Asking the agent to propose an environment (Devin's suggestion cards).
- Running checks as a delivery gate.
