# w4 · m337 — A service's "Display name changed" event names neither name, and its custom-domain rows name no domain

**Worker:** worker4 **Goal:** a rename event names the old and new names, and every custom-domain added/removed/verified event names its hostname, on REST, GraphQL, MCP and the Events tab. **Status:** blocked — t001–t004 and t006–t008 done (shipped); t005 live replay waits for the next production deploy, then t009 closes

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Backend: the rename's names and the domain's hostname on the audit row — **DONE** | 30m | — |
| [t002](done/t002.md) | API: `from`/`to` and `domain` on REST and MCP, the names on GraphQL — **DONE** | 20m | t001 |
| [t003](done/t003.md) | Dashboard: the rename line and the named domain titles, en and zh — **DONE** | 20m | t002 |
| [t004](done/t004.md) | Docs: the service-events contract and ADR018 — **DONE** | 5m | t002 |
| [t005](t005.md) | Live verification on an owned service | 15m | t003 |
| [t006](done/t006.md) | Render parity — **DONE** | 5m | t002 |
| [t007](done/t007.md) | Simplify — **DONE** | 15m | t004 |
| [t008](done/t008.md) | Test coverage — **DONE** | 20m | t004 |
| [t009](t009.md) | Closeout | 5m | t005–t008 |

Nine tasks, 135m (2h15m).

## Source + Goal linkage

- **Source:** promoted from inbox note w4/594 (live `/qa-find-bugs w4` loop, rounds 155 and 160, 2026-11-12) on 2026-11-11: two typed detail families across a migration, two recorders, the feed lookup, three API surfaces, the dashboard and a live replay.
- **Goal linkage:** the Events feed is the owner's record of what changed (ADR006 events). w4/568, w4/561, w4/m328 and w4/m335 made the neighbouring rows say what changed.
- **Render parity:** included only to keep the details in the shape Render's other event details use: `display_name_changed` and `custom_domain_*` are bex extensions, absent from Render's event enum. A create under a free-form name records no rename, as Render records none on a create.

## Finding (from w4/594)

Why: after a rename, the Events feed reads "Display name changed" and nothing else, so nobody can tell from the feed what the service used to be called. w4/568, w4/561 and w4/m328 made the neighbouring rows (instance count, plan, autoscaling, moves, env groups) say what changed. This row is the one left blank.

- **Source / severity / estimate:** live `/qa-find-bugs w4` loop, round 155, 2026-11-12 01:31–01:36 UTC (`muse.env`, workspace `bex-canary`; pin `8330744bd643`). **Minor.** ~40m (~1h with the round 160 addendum).
- **Fixture:** Starter private service `qa-20261112-r155-pserv` (`srv-dbqhbs5jiris73e60t0g`), renamed in Settings → **Edit service name** to `qa-20261112-r155-pserv-renamed` at 01:31:39. Toast `Service renamed to "qa-20261112-r155-pserv-renamed".` The service was deleted afterwards (GET 404).
- **Probe:**
  - `GET /v1/services/srv-dbqhbs5jiris73e60t0g/events?limit=8` → `["01:31:39Z","display_name_changed","{}"]`.
  - The dashboard Events tab shows `paragraph: Display name changed`, a time, and no detail line.
  - `GET /v1/owners/tea-daif693dqjvc73e7as3g/audit-logs?startTime=2026-11-12T01:31:00Z&endTime=2026-11-12T01:32:30Z` → `UpdateServiceNameEvent`, `success`, metadata `{"service":"srv-dbqhbs5jiris73e60t0g"}`.
- **Root cause:**
  - `SetDisplayName` (`lego/backend/internal/apps/service.go:5391-5420`) knows `previous` and `trimmed`, but records through `RecordAppConfigChanged(ctx, a, core.AuditVerbSetDisplayName)` (`core/audit.go:644-650`), which carries no detail.
  - `events/service.go:1237-1300` projects audit rows by type and has no `TypeDisplayNameChanged` case, so the event's details stay empty.
  - The dashboard row (`EventSummary`, `dashboard/src/routes/services.$serviceId.events.tsx:381-`) has no display-name branch.
- **Fix (target behavior):**
  - The event carries both names, and the row reads e.g. `qa-20261112-r155-pserv → qa-20261112-r155-pserv-renamed`.
  - **A cleared label** (back to the immutable name) shows the immutable name on that side.
  - **Model:** follow `RecordEnvGroupLinkChanged` (`core/audit.go:671-`, w4/563), which records a typed name detail.
  - Add two typed fields on `AuditEvent` and a store column pair with its migration. Project them on REST (`details.from` / `details.to`, the shape `branch_changed` already uses at `events/rest.go:303-305`), GraphQL (`displayNameFrom` / `displayNameTo`), MCP's event list and the dashboard row. en and zh.
  - Display names are non-secret labels shown to every reader of the service. Extend `AuditEvent`'s comment (`core/audit.go:38-47`, above `type AuditEvent` at `:48`), which lists the maintenance URI as the only tenant-chosen string.
- **Blast radius:**
  - The one verb `apps.SetDisplayName`.
  - The free-form-name create path (`apps/service.go:2163`) calls `SetDisplayName` right after a create, so a create with a free-form name also gets a named row. Decide whether that row should read as a rename at all; it is new on the board.
  - Rows recorded before the fix keep `{}`, and the dashboard row must render them as today.
- **Unverified:**
  - Whether MCP's event list passes details through unchanged.
  - The create-with-label path's row, which was not probed this run.
- **Render:** n/a. `display_name_changed` is a bex extension: Render's service event enum (pinned OpenAPI, 70 types) has no rename event. Render's `UpdateServiceNameEvent` audit metadata was not captured.
- **Dedupe:**
  - w4/568 (done; `c7b8f5a44` not yet in the pin) covers instance-count, autoscaling and plan rows only.
  - w4/m122 (done) left domain rows unnamed as a follow-up, which is a different verb.
  - w6/054 (done) noted that the feed records renames.
  - `grep -rli "displayNameFrom\|display_name_changed" .pm` finds no open item.
  - Not deploy lag: `git log 8330744bd..origin/main -S"TypeDisplayNameChanged" -- lego/backend` is empty.
- **DoD (live):** rename an owned `qa-` service in `bex-canary`:
  - `GET …/events` shows `display_name_changed` with `details.from` / `details.to` holding the two names;
  - the dashboard row reads `<old> → <new>` in en and zh.

## Round 160 addendum (2026-11-12 02:53–02:56 UTC, pin `e7a56b07e0d7`): the custom-domain rows don't name the domain either

- **Fixture:** Free web service `qa-20261112-r160-dom` (`srv-dbqik6i140ds739rb7pg`, `traefik/whoami:v1.10.1`), deleted afterwards. In Settings → **Add Custom Domain**, `qa160.example.com` was added (pending), re-checked, then deleted in the same visit (`GET …/custom-domains` → `[]`).
- **Probe:** `GET /v1/services/srv-dbqik6i140ds739rb7pg/events?limit=3` returns `["02:56:25","custom_domain_removed","{}"]` and `["02:53:56","custom_domain_added","{}"]`. The Events tab reads "Custom domain added" with no hostname.
  - On a service with several domains, the feed can't say which one was added, removed or verified.
- **Same mechanism as the rename:** `apps.AddDomain` / `apps.DeleteDomain` / `apps.VerifyDomain` map to `custom_domain_added` / `_removed` / `_verified` (`lego/backend/internal/events/service.go:443-445`), and their audit rows carry no hostname detail.
  - w4/m122 t002 (done) left exactly this as a follow-up ("naming the domain needs a new persisted detail column").
- **Widened target:**
  - the three domain rows carry the hostname (e.g. `details.domain`) on REST, GraphQL and MCP;
  - the dashboard row reads "Custom domain added · qa160.example.com".
  - Render's service event enum has no `custom_domain_*` types (pinned OpenAPI), so these are bex extensions with no Render shape to match.
- **Estimate:** now ~1h for the two renames and the three domain rows.
- **DoD addition:** add and delete a throwaway `qa-` hostname on an owned service. The two event rows name it, in en and zh.
- **Also seen, all correct:**
  - adding the service's own `<slug>.onbex.co` is refused inline with "… is already this service's platform subdomain, so it can't be added as a custom domain" (w4/558, deployed);
  - `*.qa160w.example.com` is refused with "Wildcard custom domains are not available on this platform yet" (w4/m326's delegation-zone gate);
  - the DNS dialog gives CNAME `qa160` → `qa-20261112-r160-dom.onbex.co`;
  - Re-check answers "qa160.example.com isn't verified yet — DNS may still be propagating";
  - the delete dialog says nothing was served and no certificate was issued.

## Post-release probe (QA loop round 175, 2026-11-12): t005 passes

- **Pin:** `10acf5314735`, deployed 06:46 UTC.
- **Fixture:** Starter web service `qa-20261112-r175-web` (`srv-dbqm93s5ph4s73f9gpbg`), deleted afterwards (DELETE 204, then GET 404).
- **Rename:** in Settings → General, **Edit service name** → `qa-20261112-r175-renamed` → **Save changes**.
  - The toast reads `Service renamed to "qa-20261112-r175-renamed".`
  - REST `display_name_changed` has `{"from":"qa-20261112-r175-web","to":"qa-20261112-r175-renamed"}`.
  - GraphQL `serviceEvents { details { displayNameFrom displayNameTo } }` returns the same two names.
- **Domains:**
  - `POST …/custom-domains {"name":"qa-20261112-r175.example"}` → 201. It created the apex and `www.qa-20261112-r175.example`, both `unverified`.
  - `DELETE …/custom-domains/qa-20261112-r175.example` → 204 removed both, and the list read `[]`.
  - REST lists four rows, each with `{"domain":…}`: `custom_domain_added` for the apex and for www, then `custom_domain_removed` for both. GraphQL `details.domain` agrees.
- **Events tab:**
  - en: "Name qa-20261112-r175-web → qa-20261112-r175-renamed", "Custom domain qa-20261112-r175.example added" and "Custom domain www.qa-20261112-r175.example removed".
  - zh: "名称 qa-20261112-r175-web → qa-20261112-r175-renamed", "已添加自定义域名 qa-20261112-r175.example" and "已移除自定义域名 www.qa-20261112-r175.example".
