# w6 · m50 — Blueprint sync failures vanish; global search fuzzy-matches noise from resource IDs

**Worker:** worker1 **Goal:** a blueprint sync failure leaves a diagnosable trail instead of a bare `error` state, and the workspace-wide search box actually narrows results for realistic short queries **Status:** done — code complete and gated green; live verification carried to `w6/040` (blocked on a broken deploy pipeline)

## Tasks (in order)

| id | title | est | depends_on | status |
| --- | --- | --- | --- | --- |
| t001 | Persist the blueprint sync failure reason (schema + store) | 30m | — | — **DONE** |
| t002 | Thread the persisted error through REST/GraphQL/MCP | 30m | t001 | — **DONE** |
| t003 | Surface the error in the dashboard's Sync History table | 20m | t002 | — **DONE** |
| t004 | Global search: replace cmdk's default fuzzy filter with literal substring matching | 20m | — | — **DONE** |
| t005 | Render parity | 20m | t002, t003, t004 | — **DONE** |
| t006 | Simplify | 20m | t005 | — **DONE** |
| t007 | Test coverage | 30m | t005 | — **DONE** |
| t008 | Closeout | 10m | t006, t007 | — **DONE** |

## Definition of done

- A blueprint sync that fails (manual REST/GraphQL/MCP `Sync`, or webhook-triggered auto-sync) writes a human-readable reason into `blueprint_syncs`, retrievable after the fact — not only in the synchronous response to whoever triggered that one call.
- `GET`/GraphQL/MCP reads of a blueprint's sync history include that reason for every `error`-state row.
- The dashboard's Sync History table (`blueprints.$blueprintId.tsx`) renders the reason for an `Error` row — verified live against `blp-d9nqg95cavls73fp8m10` (`discourse_docker`), which has 9 pre-existing unexplained `Error` rows spanning 2026-08-04 through 2026-08-21.
- Typing a short, realistic query into the workspace's Cmd+K global search (e.g. `db`, `cms`) returns only resources whose name/id/type actually contain that substring — verified live: `db` currently returns 9 items including two Projects and a Key Value with no `db` substring anywhere in their name, id, or type; `cms` currently returns 9 items though only 2 (`beancount-cms-v2`, `eden-cms-v2`) contain `cms`. Both must narrow to exactly their true substring matches after the fix.
- Exact-ID search continues to work (typing a resource's full or partial id still finds it) — the fix must not regress the id-search use case `value={name} ${id} ${type}` currently (accidentally) provides.

## Source + Goal linkage

- **Source:** live `/qa-find-bugs` hunt of `dashboard.bex.co` hosting surfaces, 2026-08-23 (run from a `/loop 30m` session, adversarial-testing angle: copy-to-clipboard accuracy — clean, no bug — then blueprint validation/sync-history inspection and global-search characterization, both of which surfaced real findings).
- **Goal linkage:** [docs/ADR049-render-yaml-parity.md](../../../docs/ADR049-render-yaml-parity.md) (blueprint sync is a core IaC surface — a silent, unexplained failure defeats the whole point of "sync shows you what happened") and the workspace-wide search box is a primary navigation surface used on every page.
- **Expected outcome:** an operator whose auto-sync starts failing (the most common real-world trigger — a git push that breaks the manifest, a datastore rename, a quota hit) can see _why_ without cluster/log access, the same way a failed deploy already tells you why. A user typing a plausible short query into search gets a genuinely narrowed list instead of near the entire workspace.
- **Why now:** the blueprint bug has been silently swallowing errors since the sync-history feature shipped (`w2/m62`, git-connected blueprints) — `blp-d9nqg95cavls73fp8m10` already carries 9 unexplained failures over 3 weeks in production with zero forensic trail, and every future auto-sync failure adds another. The search bug is the second confirmed instance of the exact same failure mode (`w9/m92/t002` fixed an unrelated hand-rolled fuzzy matcher on the Agents sidebar for the identical "search doesn't narrow" symptom) — a different mechanism (cmdk's bundled `command-score` scorer vs. a hand-rolled subsequence fallback) but the same lesson, and the fix pattern (`shouldFilter={false}` + explicit substring `.includes()`) already exists correctly implemented in this same codebase at `dashboard/src/common/components/ui/combobox.tsx:63-71` — this is a mechanical port, not new design work.
- **Render parity:** included (t005) — this milestone touches REST, GraphQL, MCP (blueprint sync read/write) and the dashboard UI (Sync History table + global search).
