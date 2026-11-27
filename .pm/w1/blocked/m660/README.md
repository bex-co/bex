# w1 · m660 — A static site's Confirm Rollback page compares the build's environment, as Render's does

**Worker:** worker1 **Goal:** a static site's Confirm Rollback page lists the env var, secret file and environment group changes since the target build, in Render's static wording, instead of always saying "Unable to load configuration for the current live deploy…". **Status:** blocked (pin must carry `78029e9282`, then the rest of t004: the Confirm Rollback page's frames on production at 1440 and 390 px, and t008; t001–t003 and t005–t007 done; t004 walked DoD 1–2 on production pin `410850e80281`, where the frame reserved no notice and the page grew by its comparison after mounting, fixed in `78029e9282`)

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

## Unblock (2026-11-26)

- **Affects:** t004 (its production walk), then t008.
- **Source:** verified evidence. Production's newest pin, `chore(deploy): pin platform images to 410850e80281` (`918667a83f`), carries `303185a725` (`git merge-base --is-ancestor 303185a725 410850e80281` exits 0), so the gate `pin must carry 303185a725` holds.
- **Next:** a worker walks t004's production steps as the QA user in `bex-canary` on a disposable static site, checks the Confirm Rollback page's pending frame against its ready state at 1440 and 390 px with and without changes listed, deletes the site, and runs t008. No user action.

## Production walk and the rollback page's frames (2026-11-26)

t004 walked production pin `410850e80281` as the QA user in `bex-canary` on the disposable static site `qa-20261126-m660-st` (disposable), deleted afterwards ([t004](t004.md#production-walk-2026-11-26-pin-410850e80281)). DoD 1 and 2 hold, and DoD 4 rests on t003's tests of the eight locales. DoD 3 could not be walked, because the QA user's three workspaces hold no static site and so no static deploy from before 2026-10-19. The frames did not hold:

- **What failed.** The route's frame reserved none of the notices, and a static site's ready section always shows its redirects notice. The buttons dropped 65 px at 1440 and 88 px at 390 when the page mounted. The page then mounted over the comparison's spinner frame. When the comparison listed changes, the page grew again when the read landed, by 211 px at 1440 and 475 px at 390. This is the failure w1/m657, m658 and m659 met on production: a frame shorter than what replaces it.
- **Decision (worker, 2026-11-26; basis: the root `AGENTS.md` rule and `dashboard/AGENTS.md` § Navigation pending states, as w1/m659 decided for the deploy page's Notices).**
  - The rollback routes (`/services` and `/static`) read the comparison in their loader, beside the deploy page's reads (`prefetchRollbackPage`). The read is fresh on entry and on hover, since it compares with whatever is live now, and a retained match answers from the cache. The page reads it `cache-first`, so it mounts with the comparison drawn and never moves after mounting.
  - The route's frame reserves the notices the ready section shows, in their own boxes and copy: a static site's always, and the Auto-Deploy warning when the cached service has auto-deploy on (`DeployRollbackSkeleton` reads it as `DeployHistorySkeleton` does).
  - The frame still reserves the empty comparison's height (148 px). That is the representative count the rule allows for a data-dependent region. It matches a rollback with nothing to revert, the common case for a code rollback. A rollback with changes adds its rows on the one swap from the frame.
- **Not chosen.** Reserving a representative count of comparison rows, which would shift every rollback with no changes upward instead. Keeping the page's `network-only` read, which mounts the spinner and then grows.
- **dev-1.** With the dashboard from this tree, on the static site `m660-frames` (disposable) with auto-deploy on and the comparison held, the section matches its ready state at both widths when nothing changed. The card is 231 px at 1440 and 251 px at 390 in both states, the notices are 49 and 71 px at 1440 and 72 and 94 px at 390 in both, and the buttons sit 391 and 456 px below the card's top in both. With changes the page mounted with its rows drawn and did not move again. The swap added the rows: 74 px at 1440, 170 px at 390.
- **Filed:** [w1/172](../../172.md), the deploy header's one-line title frame. At 390 the walk's 150-character commit title wrapped onto four lines, so the card sat 78 px lower once the page landed. It is shared with the deploy page.

## Parked (2026-11-26)

**Gate:** pin must carry `78029e9282`. Nothing is needed from the user. Then a worker runs [t004 § Next](t004.md#next-once-the-pin-carries-the-fix) and t008.
