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
  id            text primary key,     -- skl-<xid>, id.New(id.AgentSkill)
  workspace_id  text not null,
  name          text not null,        -- unique per workspace
  description   text not null,
  triggers      text[] not null,      -- {'user'} or {'user','model'}
  enabled       boolean not null,
  body          text not null,
  extra         text not null,        -- every other frontmatter key, as YAML
  version       int  not null,        -- +1 on every update
  updated_by    text not null,
  created_at    timestamptz not null,
  updated_at    timestamptz not null,
  unique (workspace_id, name)
)
```

Migration 0204 creates the table with the workspace as a cascading foreign key and adds `agent_session_turns.skills` (D4). Account deletion anonymizes `updated_by` like the other creator columns.

Limits follow the Agent Skills format where it sets one:

- `name`: `^[a-z0-9]+(-[a-z0-9]+)*$`, at most 64 characters.
- `description`: one line of 1–1024 characters, with no control characters or Unicode line or paragraph separators. The agent reads it in the overlay index on every turn.
- `body`: at most 64 KiB.
- At most 100 skills per workspace.
- At most 1 MiB of enabled skills per workspace, so one dispatch delivers at most about that. A skill counts its field bytes (name, description, body, other frontmatter) plus 64 bytes for its frontmatter scaffolding, and the sum is checked in SQL in the same transaction as the write, under a per-workspace advisory lock, so concurrent creates or enables cannot pass it together. A create, update or enable past it is refused. The dispatch re-checks the rendered documents as a guard.

A breach answers `400 AGENT_SKILL_LIMIT_EXCEEDED` with the limit in `details`. A duplicate name answers `409 AGENT_SKILL_CONFLICT`.

Skills are instructions, not secrets: they are stored in Postgres and readable by any workspace member. Credentials belong in session secrets ([ADR098](ADR098-agent-session-secrets.md)); the dashboard says so next to the editor.

Version history is out of scope. `version` gives optimistic concurrency (`ifVersion` on update → `409` on mismatch) and an audit trail through the existing audit log. Devin keeps per-save history for playbooks; that is filed as a follow-up.

### D3 — API: one core, three thin adapters

| Operation | REST | GraphQL | MCP (`/mcp/cloud-agents`) |
| --- | --- | --- | --- |
| list | `GET /v1/agent-skills?ownerId=` | `agentSkills(ownerId)` | `list_agent_skills` |
| get | `GET /v1/agent-skills/{ref}` | `agentSkill(ownerId, skill)` | `get_agent_skill` |
| create | `POST /v1/agent-skills?ownerId=` | `createAgentSkill(ownerId, input)` | `create_agent_skill` |
| update | `PATCH /v1/agent-skills/{ref}` | `updateAgentSkill(ownerId, skill, input)` | `update_agent_skill` |
| delete | `DELETE /v1/agent-skills/{ref}` | `deleteAgentSkill(ownerId, skill)` | `delete_agent_skill` |

`{ref}` (GraphQL and MCP `skill`) is a `skl-` id or a name. An id finds its workspace from the row, like Render's by-id routes; a name is looked up in `ownerId` (or the caller's workspace). A skill of another workspace reads as not found.

A list carries the metadata only (name, description, triggers, enabled, version). `body` and `content` are on single reads, so a list of 100 skills stays small. A write takes either the fields or `content`, not both; `enabled` combines with either.

**Authorization.**

- Members with `can_create` on the workspace list, read and use skills, the same relation as creating a session.
- `can_manage` (admins) create, update and delete them. This mirrors Devin's "Manage Knowledge" permission and bex's model-key and secrets rules.
- The MCP read tools sit in the `bex.cloud_agents` scope; write tools need the workspace-admin scope used by the agent-environment tools.

### D4 — Explicit invocation on create and steer

Create and steer take `skills: [{name, arguments?}]`, at most 5 per turn. This is the API spelling of Devin's `playbook_id` and of `@skills:<name> <args>`.

- The prompt may also say `@skills:<name> args` inline. The driver leaves the text as is; the engine sees it and the overlay explains it.
- A named skill must exist and be enabled, or the request is refused `404 AGENT_SKILL_NOT_FOUND`: the same code and status as a skill read, as ADR098 does for a missing secret reference. A `["user"]`-only skill can be invoked this way.
- The turn records the invoked names and arguments (`agent_session_turns.skills`). The transcript's turns show them (`skills` on each turn), and so does the dashboard pill.
- The idempotency fingerprint includes the invoked names and arguments (ADR094 D4).
- Arguments are one line of at most 4 KiB, with no control characters, so they cannot add lines to the prompt (D5). A skill's body with its arguments substituted is at most 128 KiB; an invocation past that is refused `400 AGENT_SKILL_LIMIT_EXCEEDED`, and a dispatch whose skill grew past it since fails the turn with `SKILL_TOO_LARGE`. The size is computed before the substitution is built.
- A deployment that cannot deliver skills (no database or no sandbox file copy) refuses an invocation `503 AGENT_SKILLS_UNAVAILABLE` instead of accepting one it would drop.

### D5 — Delivery at dispatch: files on disk plus an index in the overlay

At every dispatch, bex-api snapshots the workspace's **enabled** skills, plus any skill invoked this turn even if it was disabled since, and delivers them into the sandbox. A `["user"]` skill is delivered too, so a prompt can invoke it inline; its delivered copy carries Claude Code's `disable-model-invocation: true`, forced true whatever the stored frontmatter says, so that engine does not apply it on its own. An invoked skill that was deleted since is skipped.

Delivery reuses ADR095 D5: one tar of `<name>/SKILL.md` entries, copied through a platform ticket bound to `/var/lib/bex-agent/skills` (`sandboxfiles.AgentSkillsPath`, the second destination such a ticket may name). The image links `/bex/skills` to that path. `BEX_AGENT_SKILLS` lists the names, the user-only flags, this turn's invocations with their arguments, and each skill's description for the overlay index, shortened to 200 characters and to 8 KiB across all skills (later skills go without one). That keeps the env var far under the kernel's per-string limit, and the driver never parses YAML. A workspace without skills costs the dispatch one metadata query; the turn row is read only when a skill exists, and once for attachments and skills together.

bex-api substitutes `$ARGUMENTS` and `$1`–`$9` in the delivered copy of each skill this turn invokes, so engines that do not substitute still see the values. A skill the turn does not invoke keeps its placeholders.

The driver then:

1. Links each skill into every engine's native user folder (`~/.agents/skills`, `~/.claude/skills`, `~/.codex/skills`), so engines with native skill support load it themselves. On a name clash, a repo skill shadows a workspace skill: the checkout is the more specific source, as in Devin. A folder that is a symlink or a file is skipped rather than followed, and a failed link skips that link, not the turn.
2. Adds an overlay section listing each delivered skill's name, description and path, at most 8 KiB. An engine without native skill support can still read the file on demand, which keeps the format's progressive disclosure: names up front, body when used. Claude Code and Codex already see the descriptions through their own loaders, so for them the section lists names and paths only.
3. For each skill invoked this turn (D4), prepends one line to the dispatching turn's prompt: "Use the skill `<name>` (`/bex/skills/<name>/SKILL.md`) with arguments `"<args>"`", the arguments quoted as one JSON string.

The snapshot is taken per dispatch, so an edit applies from the next turn. A running turn keeps its copy.

### D6 — Dashboard

`/agents` gains a **Skills** page in the workspace agent settings: list, enable toggle, editor (name, description, an "only when I ask" toggle for `triggers`, and a markdown body), plus import and export as `SKILL.md`. Admins edit; others read.

The new-session composer gains a skill picker next to secrets and attachments. The chosen skills appear as pills on the composer and on the turn in the transcript.

Skeletons match the ready geometry (AGENTS.md rule).

## Consequences

- Devin parity in today's model: workspace skills, repo skills, automatic plus explicit invocation, and API invocation. Knowledge and Playbooks are covered because Devin itself folded them into Skills.
- Existing repo skills keep working without bex involvement. Workspace skills reach engines through their own skill loaders, with the overlay index as a fallback.
- Every enabled skill costs prompt tokens for its index line on every turn. The 8 KiB index cap bounds this, engines with native loaders get names only, and disabled skills cost nothing.
- Dispatch failure codes: `SKILL_LIST_FAILED` (the skills could not be read), `SKILL_COPY_FAILED` (the copy into the sandbox failed) and `SKILL_TOO_LARGE` (D4).
- **Follow-ups, not gates:**
  - per-save version history (Devin playbooks keep one);
  - "Devin writes a skill from this session" (suggested skills);
  - enterprise or cross-workspace skills and plugins;
  - the `allowed-tools` and `permissions` frontmatter, which depend on approval mode (w9/m176). Until then bex stores them in `content` but does not enforce them.
