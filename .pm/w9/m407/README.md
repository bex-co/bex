# w9 · m407 — Repository environment phase 2: sessions boot from a snapshot built from the setup steps, as Devin's do

**Worker:** worker9 **Goal:** a repository's setup steps run once in a build, the result is kept as an image, and every session on that repository boots from the newest successful build with the latest code pulled and no setup wait, as Devin's sessions boot from blueprint snapshots. **Status:** todo (t001, t002, t003, t004, t005, t006, t007, t008, t011, t012 done)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Design: ADR for phase 2 — builds, triggers, statuses, secrets, boot and retention; amend ADR109 D2/D3 — **DONE** | 60m | — |
| [t002](done/t002.md) | Security review: builds make the privileged commit Job user-triggerable (ADR104 D4) — **DONE** | 45m | t001 |
| [t003](done/t003.md) | Store: environment versions and the builds table — **DONE** | 60m | t001 |
| [t004](done/t004.md) | Sandbox seams: boot an agent sandbox from a build image, commit a builder, keep its image — **DONE** | 75m | t001 |
| [t005](done/t005.md) | Driver: a build mode and a prebuilt boot mode — **DONE** | 60m | t001 |
| [t006](done/t006.md) | Build orchestration: triggers, queue, worker and failure handling — **DONE** | 90m | t002, t003, t004, t005 |
| [t007](done/t007.md) | Dispatch: sessions boot from the newest matching build; phase 1 is the fallback — **DONE** | 45m | t006 |
| [t008](done/t008.md) | Surfaces: builds on REST, GraphQL, MCP and the dashboard — **DONE** | 75m | t006, t007 |
| [t009](t009.md) | Live check on production: a build runs, the next session boots from it, and a change rebuilds | 45m | t008 |
| [t010](t010.md) | Devin parity: every surface this milestone touched agrees, and matches Devin's builds | 20m | t009 |
| [t011](done/t011.md) | Simplify — **DONE** | 15m | t010 |
| [t012](done/t012.md) | Test coverage — **DONE** | 45m | t010, t011 |
| [t013](t013.md) | Closeout | 10m | t012 |

## Definition of done

On production (QA user):

- Saving a repository environment starts a build; its status and log show on REST, GraphQL, MCP and the dashboard, and "Build now" and a daily refresh start one too.
- The next session on that repository boots from the newest successful build: setup does not run, its effect is on disk, and the checkout is at the latest base.
- A changed environment rebuilds; until a build matching it succeeds, and after a failed build, sessions fall back to phase 1 (ADR109 D2).
- The build path meets ADR104 D4's bounds, reviewed in the security lineage; ADR109 D2's divergence is closed in the docs.

## Source + Goal linkage

- **Source:** promoted from w9/096 (2026-11-14), the follow-up w9/m387 t001 filed for ADR109 D2.
  - Devin: sessions boot from the latest successful build's snapshot; builds run on a blueprint save, a repository secret added, a manual "Build snapshot" and about every 24 h; one at a time; "the latest code is pulled" at session start and `maintenance` is not rerun ([Blueprints § Builds and sessions](https://docs.devin.ai/onboard-devin/environment/blueprints), checked 2026-11-14).
  - bex: phase 1 runs setup in the driver at every turn's start (`lego/agent-image/driver/src/session.ts:270-286`). The pieces exist: agent sandboxes share the user-sandbox substrate (`lego/backend/internal/sandbox/service.go:1042`), the pause path commits a sandbox to the workspace registry (`client.go:490`), and imported images boot sandboxes from `snapshots/<ws>-sandbox/images/<id>` (`images.go:829-872`). No milestone blocks it; the privileged commit Job becoming user-triggerable needs ADR104 D4's review (t002).
- **Goal linkage:** cloud coding agents match Devin (user preference 2026-10-30, ADR047); closes ADR109 D2's recorded divergence.
- **Expected outcome:** a repository with setup steps starts each turn without the setup wait.
- **Why now:** phase 1 (w9/m387) shipped 2026-11-14; every turn on a repository with setup pays its time until this lands.
- **Render parity:** replaced by Devin parity (t010): the surfaces are agent features, whose reference product is Devin.
