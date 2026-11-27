# w1 · m691 — A service's Shell always opens on a named instance, picked as Render's picker picks it

**Worker:** worker1 **Goal:** the Shell page attaches to one named instance, chosen, kept in the URL and announced as Render's Shell does, and its `ssh` command names that instance. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](t001.md) | Record Render's Shell instance rules in `docs/render-artifacts/shell-page.md` | 30m | — |
| [t002](t002.md) | The picker: no "Any ready instance", Render's default, a read-only picker with Render's tooltip for one instance | 1h30m | t001 |
| [t003](t003.md) | The URL `/services/{id}/shell/{suffix}`, which Render's link and bex's own SSH into instance links open | 1h | t002 |
| [t004](t004.md) | The "{instance} is not available. Now connected to {instance}." toast | 45m | t002 |
| [t005](t005.md) | The `ssh` command names the selected instance | 30m | t002 |
| [t006](t006.md) | Walk a 2-instance and a 1-instance service on dev-1 and production | 45m | t003, t004, t005 |
| [t007](t007.md) | Render parity | 20m | t006 |
| [t008](t008.md) | Simplify | 15m | t007 |
| [t009](t009.md) | Test coverage | 30m | t007 |
| [t010](t010.md) | Closeout | 10m | t008, t009 |

Total: **10 tasks, about 6h15m**.

## Definition of done

1. The Shell page never offers "Any ready instance". It opens on the instance its URL names when that instance is running, and otherwise on the first running instance.
2. A service with more than one running instance keeps the selected instance in the path, `/services/{id}/shell/{suffix}`. A service with one shows a read-only picker with Render's tooltip, and its URL stays `/services/{id}/shell`.
3. Render's `/web/srv-…/shell/{suffix}` link, and bex's Logs and Metrics SSH into instance, open the Shell on that instance. The old `?instance=` form still works.
4. A URL naming an instance that is not running, or a selected instance that stops, shows Render's toast and connects to the first running instance.
5. The page's `ssh` command is `ssh srv-…-{suffix}@…` for the selected instance, and it connects to that pod.
6. All of the above walked on production at 1440 and 390 px, in all eight locales.

## Source + Goal linkage

- **Source:** `/pm-brainstorm one aspect of render parity to w1` (2026-11-26), item 1, verified before filing.
  - Render's public dashboard bundle was read 2026-11-26 without sign-in (entry `index-D5V4ZgbE.js`, 459 chunks). Chunk hashes change with every Render deploy, so find the code by the names and strings quoted here.
  - `ServiceShellStarter` defines the Shell page.
  - `server-instances` chooses the instance: of the instances whose status is running, the URL's one when listed, else the first. It reports a "not available" instance when the URL's instance is not running at load, or when the selected one disappears.
  - `urls` gives the URL suffix as the id's last `-` part.
  - `useSSHAddress` builds `${serviceId}-${instance}@ssh.${region}.render.com`.
  - No flag gates any of it. The page's only flag is `shell_local_echo`, which the ledger already records as flagged.
- **bex today (checked 2026-11-26):**
  - `dashboard/src/features/services/components/web-shell-panel.tsx` defaults to "Any ready instance" (`ANY_INSTANCE`, `:18`, `:49`, `:92`). It quietly falls back to it when the selected instance is gone (`:66-71`).
  - The route takes `?instance=` (`dashboard/src/routes/services.$serviceId.shell.tsx:15`).
  - Render's `/web/srv-…/shell/{suffix}` redirects to `/services/srv-…/shell/{suffix}` (`dashboard/src/common/lib/render-alias.ts`), which no route serves.
  - `service-shell-page.tsx:43` prints `ssh {sshAddress}` with no instance.
  - The gateway already takes a complete instance id as the SSH user ([ADR035](../../../docs/ADR035-ssh.md) step 5).
- **Not a duplicate of:**
  - w2/m55, which built the picker with "any" as its default.
  - w3/m528, which added the log line's SSH into instance link.
  - w4/501, which gave the picker its accessible name.
- **Goal linkage:** Render parity first ([ADR008](../../../docs/ADR008-vision.md#render-parity-first)).
- **Expected outcome:** the Shell chooses, shows and remembers an instance as Render's does, and Render's own deep links to an instance work on bex.
- **Why now:** Render's links to an instance land on a page that does not exist. [w1/m692](../m692/README.md) (reconnect) builds on this milestone's not-available rule.
- **Render parity task:** included (t007). The dashboard Shell page and its `ssh` command change. REST, GraphQL and MCP do not.
