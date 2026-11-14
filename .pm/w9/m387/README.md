# w9 · m387 — Repo environment: declared setup and checks run before the agent starts (Devin blueprint, phase 1)

**Worker:** worker9 **Goal:** a workspace declares, per repository, setup steps and lint/test/build checks; every turn runs the setup before the agent starts and tells the agent the checks, as Devin's blueprint `initialize`/`maintenance` steps and `knowledge` lint/test/build entries do. Phase 1 runs the setup at turn start; prebuilt snapshots are phase 2. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](t001.md) | ADR: per-repository setup steps and checks (amends ADR047 D5); phase 2 snapshots filed | 90m | — |
| [t002](t002.md) | Store and core: per-repository environment rows and validation | 90m | t001 |
| [t003](t003.md) | REST, GraphQL and MCP: read, set and delete a repository's environment | 120m | t002 |
| [t004](t004.md) | Driver: run the setup before the agent each turn, with secrets, a time limit and redacted output | 150m | t003, w9/m385/t003 |
| [t005](t005.md) | Setup failure: `AGENT_SESSION_SETUP_FAILED` names the step, exit code and output tail; the agent does not start | 60m | t004 |
| [t006](t006.md) | Overlay: list the declared checks so the agent runs them before delivering | 45m | t004 |
| [t007](t007.md) | Dashboard: Agent settings → Environment, per repository | 120m | t003 |
| [t008](t008.md) | Live check on production: setup installs dependencies before the agent and the agent runs the declared checks | 60m | t005, t006, t007 |
| [t009](t009.md) | Render parity (Devin): every surface this milestone touched agrees, and matches Devin | 20m | t008 |
| [t010](t010.md) | Simplify | 15m | t009 |
| [t011](t011.md) | Test coverage | 45m | t009, t010 |
| [t012](t012.md) | Closeout | 10m | t011 |

## Definition of done

On production (QA user):

- With a repository environment (a setup step and a `test` check) for the QA repository, a new session's transcript shows setup finishing before the agent's first message, and the agent runs the declared check before delivering and names its result.
- A repo-scoped secret (m385) is available to setup steps and its value never appears in the transcript.
- A failing setup step ends the turn with `AGENT_SESSION_SETUP_FAILED`, naming the step and exit code, on REST, GraphQL, MCP and the dashboard; the agent never starts.
- The repository environment reads and writes identically on REST, GraphQL, MCP and the dashboard.
- The ADR records phase 1, its divergence from Devin's snapshots, and the filed phase-2 note.

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w9` (2026-11-13), item 3, filed by `/pmx` 2026-11-13.
  - Devin: blueprints — `initialize` and `maintenance` run at snapshot build; `knowledge` holds `lint`/`test`/`build`, which Devin uses to verify its work; builds rerun when the blueprint changes and about every 24 h; every session boots from the snapshot ([Environment](https://docs.devin.ai/onboard-devin/environment), [Blueprint reference](https://docs.devin.ai/onboard-devin/environment/blueprint-reference), checked 2026-11-13).
  - bex: the agent environment is network tier, custom hosts and model only (`lego/backend/internal/agentsessions/environment.go:67-80`); every turn runs in a fresh sandbox (`docs/ADR098-agent-session-secrets.md:10`); re-clone plus reinstall costs about 30–60 s (`docs/ADR059-agent-sandbox-hibernation.md:157`). Nothing runs per-repository setup (`lego/agent-image/driver/src/install-profiles.ts` runs only at image build).
- **Goal linkage:** ADR008 AI-native pillar; Devin parity (ADR047); agent quality and model cost.
- **Expected outcome:** the agent starts with dependencies installed and verifies its work with the repository's own checks instead of rediscovering them each turn.
- **Why now:** agent hibernation was retired on 2026-10-25, so every turn starts from scratch; the eight daily `rt-*` automations turned on 2026-11-11 pay for that rediscovery on every run. Depends on m385 for repo-scoped secrets in setup steps.
- **Open sign-off (divergence):** phase 1 runs setup at each turn's start, where Devin boots from a prebuilt snapshot. The outcome (dependencies present, checks known) matches; the per-turn wait does not. Under ADR008 parity-first this ships as the parity-sufficient first step, and t001 files phase 2 (snapshots) as its own item.
- **Render parity:** included (t009), against Devin, because the change touches REST, GraphQL, MCP and the dashboard.
