# w1 · m159 — Dashboard truth: a datastore's own Status row, the landing after "Move to project", and seven count strings

**Worker:** worker1 **Goal:** the dashboard never contradicts itself about a resource's state or its location. A suspended Key Value or Postgres reads Suspended in every row that names its status, a resource moved into a project opens on the environment it actually landed in, and every `{count}` message reads correctly at one. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | The Key Value and Postgres Details cards translate status and plan instead of printing wire values | 40m | — |
| t002 | "Move to project" lands on the environment the resource is actually in | 45m | — |
| t003 | Seven `{count}` strings get native plurals, and the locale test fails when a count message has none | 40m | — |
| t004 | Render parity | 20m | t001, t002, t003 |
| t005 | Simplify | 15m | t004 |
| t006 | Test coverage | 40m | t004 |
| t007 | Closeout | 10m | t006 |

## Definition of done

Each bullet is observable on `https://dashboard.bex.co` with throwaway `qa-<yyyymmdd>-` fixtures, and on a local dev build for the locale half.

- **A suspended datastore reads suspended everywhere.** Create a free Key Value store, suspend it, and open its page: the header badge and the Details card's Status row both read Suspended, and Instance type reads the plan's display name. At filing time the header read **Suspended** while the Details row read **available** and the plan read **free**. The same holds for a Postgres instance, and in both locales.
- **A moved resource is visible where it landed.** "Move to project" on an ungrouped resource row, then open the project: the environment picker selects the environment that holds the resource (Unassigned when that is where it landed), and the resource is on screen. At filing time the page opened on an empty environment while the resource sat under Unassigned.
- **Counts read correctly at one.** `/blueprints/new` with `examples/hello-go/render.yaml` reads "1 resource to sync" (control: `examples/stack-demo/render.yaml` reads "3 resources to sync"), and no user-visible string carries `(s)`.
- **The rule is enforced from now on.** `dashboard/yarn test` fails when an `en` message containing `{count}` has no `_one`/`_other` form.

## Blast radius

- **Who is hit.** Every datastore detail page, every "Move to project" from a resource row, and seven strings across Blueprints, env groups and services.
- **Severity.** Minor each, but all three are truthfulness defects a user meets immediately after a successful action.

## Source + Goal linkage

- **Source:** `w1/085`, `w1/086` and `w1/098` — live `/qa-find-bugs` passes 2, 4 and 26 (2026-09-14). Promoted 2026-09-15 during the w1 triage, grouped because each is sub-hour on its own and they share one review and one ship.
- **Goal linkage:** `docs/ADR018-render-parity.md` (the dashboard is a parity surface) and `dashboard/CLAUDE.md` — a view must describe the state it is actually showing.
- **Expected outcome:** no dashboard row contradicts the resource it describes, and no count message reads "1 resources".
- **Why now:** all three were found live and none needs a decision; they are the cheapest user-visible wins left in w1's inbox.
- **Render parity is included** because all three are user-facing dashboard surfaces; `w6/done/062` set the plural rule this restores.
