# ADR099 — Workspace skills for cloud agents (Knowledge and Playbooks, Devin parity)

**Status:** accepted (design, w9/m175 t001) · **Date:** 2026-11-06 · **Extends:** [ADR047](ADR047-cloud-coding-agent-sessions.md), [ADR090](ADR090-agent-owned-delivery.md) (overlay), [ADR094](ADR094-coding-agent-mcp.md) (Cloud Agents API), [ADR095](ADR095-agent-attachments.md) D5 (dispatch copy)

## Context

Teams re-type the same instructions into every cloud agent session: "run `make test` before the PR", "our staging deploy goes through X", "follow this release procedure". Devin answers this with reusable instructions. w9/m175 was filed as "Knowledge and Playbooks", Devin's two historical features.

**What Devin does today (verified 2026-11-06):**

- **Knowledge** was a name, a trigger description and a short body, retrieved when the trigger matched, optionally pinned to a repo or to all repos. It is **deprecated**: existing Knowledge is being migrated to Skills automatically ([Knowledge](https://docs.devin.ai/product-guides/knowledge)).
- **Playbooks** were prompt templates (procedure, specifications, advice, forbidden actions, required input) attached through a UI selector, a `!macro` or the API's `playbook_id`. They are **becoming Skills** too: each playbook becomes a skill with the same instructions, the Playbooks API reads and writes the skill, and accounts created from 2026-10-24 start with Skills only ([Playbooks](https://docs.devin.ai/product-guides/creating-playbooks)).
- **Skills** are `SKILL.md` files in the open Agent Skills format: YAML frontmatter with `name` and `description` (plus optional `argument-hint`, `triggers`, `allowed-tools`, `permissions`), then a body of instructions. Devin finds them in the repository as it is cloned (`.agents/skills/<name>/SKILL.md` recommended; `.claude/`, `.codex/`, `.github/` and other tool folders also scanned) and in organization-level skills written on the Customize page or installed through plugins. Only the name and description are loaded up front; the body is read when the skill is invoked, either automatically when relevant or explicitly with `@skills:<name> <args>` (`$ARGUMENTS`, `$1`–`$9`). `triggers: ["user"]` turns automatic invocation off ([Skills](https://docs.devin.ai/product-guides/skills)).

So the parity target is **Skills**: one concept that covers both of the features m175 named. Building Knowledge and Playbooks as two separate entities would copy a model Devin is retiring.

The engines bex runs already understand the format. Claude Code reads `~/.claude/skills/` and the repo's `.claude/skills/`; Codex reads its own skills folders. Repo skills therefore already work in a bex session today, on whatever branch the agent checks out. What is missing is the **workspace-level** layer, which has no repository to live in, plus explicit invocation from the API.

## Decision

### D1 — One entity: a workspace skill in the Agent Skills format

A workspace skill is a `SKILL.md` document owned by the workspace: `name`, `description`, `triggers` (`["user","model"]` default, or `["user"]`), an `enabled` flag and the instruction `body`. There is no separate Knowledge or Playbook entity.

- Devin's Knowledge "trigger description" maps to `description`, and a "pinned to all repos" item maps to an enabled skill with model triggers.
- A Playbook maps to a skill with `triggers: ["user"]` that is invoked explicitly.
- The API also accepts and returns the whole file as `content` (frontmatter + body), so a skill can move between a repository and the workspace unchanged.

Repository skills stay where they are: the engine discovers them in the checkout, as Devin does. bex does not copy or index them.

### D2 — Storage and limits

```sql
agent_skills (
  id            text primary key,     -- skl-<xid>, id.New(id.KindAgentSkill)
  workspace_id  text not null,
  name          text not null,        -- unique per workspace
  description   text not null,
  triggers      text[] not null,      -- {'user'} or {'user','model'}
  enabled       boolean not null,
  body          text not null,
  version       int  not null,        -- +1 on every update
  updated_by    text not null,
  created_at    timestamptz not null,
  updated_at    timestamptz not null,
  unique (workspace_id, name)
)
```

Limits follow the Agent Skills format where it sets one:

- `name`: `^[a-z0-9]+(-[a-z0-9]+)*$`, at most 64 characters.
- `description`: 1–1024 characters.
- `body`: at most 64 KiB.
- At most 100 skills per workspace.
- At most 1 MiB of enabled skills delivered to one dispatch.

A breach answers `400 AGENT_SKILL_LIMIT_EXCEEDED` with the limit in `details`. A duplicate name answers `409 AGENT_SKILL_CONFLICT`.

Skills are instructions, not secrets: they are stored in Postgres and readable by any workspace member. Credentials belong in session secrets ([ADR098](ADR098-agent-session-secrets.md)); the dashboard says so next to the editor.

Version history is out of scope. `version` gives optimistic concurrency (`ifVersion` on update → `409` on mismatch) and an audit trail through the existing audit log. Devin keeps per-save history for playbooks; that is filed as a follow-up.

### D3 — API: one core, three thin adapters

| Operation | REST | GraphQL | MCP (`/mcp/cloud-agents`) |
| --- | --- | --- | --- |
| list | `GET /v1/agent-skills?ownerId=` | `agentSkills(ownerId)` | `list_agent_skills` |
| get | `GET /v1/agent-skills/{idOrName}?ownerId=` | `agentSkill` | `get_agent_skill` |
| create | `POST /v1/agent-skills` | `createAgentSkill` | `create_agent_skill` |
| update | `PATCH /v1/agent-skills/{id}` | `updateAgentSkill` | `update_agent_skill` |
| delete | `DELETE /v1/agent-skills/{id}` | `deleteAgentSkill` | `delete_agent_skill` |

**Authorization.**

- Members with `can_create` on the workspace list, read and use skills, the same relation as creating a session.
- `can_manage` (admins) create, update and delete them. This mirrors Devin's "Manage Knowledge" permission and bex's model-key and secrets rules.
- The MCP read tools sit in the `bex.cloud_agents` scope; write tools need the workspace-admin scope used by the agent-environment tools.

### D4 — Explicit invocation on create and steer

Create and steer take `skills: [{name, arguments?}]`, at most 5 per turn. This is the API spelling of Devin's `playbook_id` and of `@skills:<name> <args>`.

- The prompt may also say `@skills:<name> args` inline. The driver leaves the text as is; the engine sees it and the overlay explains it.
- A named skill must exist and be enabled, or the request is refused `400 AGENT_SKILL_NOT_FOUND`. A `["user"]`-only skill can be invoked this way.
- The session records the invoked names per turn; the view shows them, and so do the dashboard pill and the transcript.
- The idempotency fingerprint includes the invoked names and arguments (ADR094 D4).

### D5 — Delivery at dispatch: files on disk plus an index in the overlay

At every dispatch, bex-api snapshots the workspace's **enabled** skills, plus any `["user"]` skill invoked this turn, and delivers them into the sandbox. Delivery reuses ADR095 D5: one tar copied through a path-bound platform ticket, here into `/bex/skills/<name>/SKILL.md`.

The driver then:

1. Links each skill into every engine's native user folder (`~/.agents/skills`, `~/.claude/skills`, `~/.codex/skills`), so engines with native skill support load it themselves. On a name clash, a repo skill shadows a workspace skill: the checkout is the more specific source, as in Devin.
2. Adds an overlay section listing each delivered skill's name, description and path. An engine without native skill support can still read the file on demand, which keeps the format's progressive disclosure: names up front, body when used.
3. For each skill invoked this turn (D4), prepends one line to the turn prompt: "Use the skill `<name>` (`/bex/skills/<name>/SKILL.md`) with arguments: `<args>`". The driver substitutes `$ARGUMENTS` and `$1`–`$9` in the delivered copy of that skill, so engines that do not substitute still see the values.

The snapshot is taken per dispatch, so an edit applies from the next turn. A running turn keeps its copy.

### D6 — Dashboard

`/agents` gains a **Skills** page in the workspace agent settings: list, enable toggle, editor (name, description, an "only when I ask" toggle for `triggers`, and a markdown body), plus import and export as `SKILL.md`. Admins edit; others read.

The new-session composer gains a skill picker next to secrets and attachments. The chosen skills appear as pills on the composer and on the turn in the transcript.

Skeletons match the ready geometry (AGENTS.md rule).

## Consequences

- Devin parity in today's model: workspace skills, repo skills, automatic plus explicit invocation, and API invocation. Knowledge and Playbooks are covered because Devin itself folded them into Skills.
- Existing repo skills keep working without bex involvement. Workspace skills reach engines through their own skill loaders, with the overlay index as a fallback.
- Every enabled skill costs prompt tokens for its index line on every turn. The 100-skill and 1 MiB caps bound this, and disabled skills cost nothing.
- **Follow-ups, not gates:**
  - per-save version history (Devin playbooks keep one);
  - "Devin writes a skill from this session" (suggested skills);
  - enterprise or cross-workspace skills and plugins;
  - the `allowed-tools` and `permissions` frontmatter, which depend on approval mode (w9/m176). Until then bex stores them in `content` but does not enforce them.
