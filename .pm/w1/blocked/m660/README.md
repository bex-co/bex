# w1 · m660 — A static site's Confirm Rollback page compares the build's environment, as Render's does

**Worker:** worker1 **Goal:** a static site's Confirm Rollback page lists the env var, secret file and environment group changes since the target build, in Render's static wording, instead of always saying "Unable to load configuration for the current live deploy…". **Status:** blocked (pin must carry `303185a725`: t004's production walk as the QA user, then t008; t001–t003 and t005–t007 done, and t004's dev-1 walk passed)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | The operator records the environment sources each static build used — **DONE** | 1h30m | — |
| [t002](done/t002.md) | `RollbackComparison` compares a static site's two records — **DONE** | 1h | t001 |
| [t003](done/t003.md) | The Confirm Rollback page's static wording, in all eight locales — **DONE** | 1h | t002 |
| [t004](t004.md) | Walk a static rollback on dev-1 and on production | 45m | t003 |
| [t005](done/t005.md) | Render parity — **DONE** | 20m | t004 |
| [t006](done/t006.md) | Simplify — **DONE** | 15m | t005 |
| [t007](done/t007.md) | Test coverage — **DONE** | 30m | t005 |
| [t008](t008.md) | Closeout | 15m | t006, t007 |

## Definition of done

1. On production, as the QA user, on a disposable static site whose env var, secret file and linked environment group changed after a deploy: **Rollback** on that deploy opens a Confirm Rollback page that lists each change in Render's static wording (env var and secret file rows by name, and the Environment Group sentences "The rollback will not use environment group **X** which was added to the site after this build", "This build used **N** environment groups that have since been deleted. The rollback will use their original values", "The rollback will use the original values for environment groups used for this build.").
2. No value appears in the page or in the GraphQL `rollbackComparison` response, only names, as for services.
3. A static release built before t001 shipped still shows Render's "Unable to load configuration…" sentence, recorded as a known limit in ADR018's Rollback row. (Amended 2026-11-26: t001 found the record already kept by every static release since w8/m120, 2026-10-19, for a built site and w8/392, 2026-11-21, for a no-build one, so the boundary is those dates, not t001's ship.)
4. Every sentence reads in all eight locales.

## Source + Goal linkage

- **Source:** promoted from [w1/156](../../done/156.md) on 2026-11-26 by `/pm`, after `/pm-brainstorm one aspect of render parity to w1` verified it: `lego/backend/internal/deploys/rollback_comparison.go:155` returns before any snapshot for a static site (its releases write only `StaticPublicationRecordName`), and `dashboard/src/features/deploys/components/rollback-comparison.tsx` has no static wording. Render: [rollback-comparison.md](../../../../docs/render-artifacts/rollback-comparison.md) and the `DeployContainer` chunk of its dashboard bundle (read 2026-11-26, entry `index-D5V4ZgbE.js`).
- **Goal linkage:** Render parity first ([ADR008](../../../../docs/ADR008-vision.md#render-parity-first)); completes w1/m646's Confirm Rollback page for the one service type it left on the fallback.
- **Expected outcome:** a user rolling a static site back sees which environment changes the rollback undoes, as on a web service and as on Render.
- **Why now:** since w1/m646 every static site's rollback page shows the "Unable to load configuration…" fallback, and records only begin with the first static build after t001 ships, so each day of delay adds releases that can never be compared.
- **Not duplicated (checked 2026-11-26):** w1/m646 (blocked on a pin for its DoD 4 skeleton re-check) built the page and left this as w1/156; no other workstream has static rollback comparison open.
- **Render parity task:** included (t005), because the milestone changes GraphQL `rollbackComparison`'s answer and the dashboard.

## Outcome so far (2026-11-26)

Shipped in `303185a725`:

- GraphQL `rollbackComparison` answers a static site's snapshots from the record each static release keeps at adoption. They carry names only, with no command, health check, instance or plan field.
- A static Rollback or Restart is compared as the build whose publication it copied.
- The dashboard's Environment Group row uses Render's static sentences in all eight locales.

The dev-1 walk passed ([t004](t004.md)). The production walk waits for a pin carrying `303185a725`.
