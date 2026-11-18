# w9 · m451 — Shell sessions appear in the workspace audit log as StartShellEvent and EndShellEvent

**Worker:** worker9 **Goal:** every SSH and dashboard Shell session into a service instance leaves Render's `StartShellEvent` and `EndShellEvent` rows, with Render's shell metadata, in the workspace audit log **Status:** todo (t001–t004 and t006–t008 done; t005's production walk found native SSH rows naming the load balancer as `source_ip`, which t010 fixes at the edge; then t005's recheck and t009)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Decide and grant the gateway's audit write path — **DONE** | 30m | w9/m450/t002 |
| [t002](done/t002.md) | Native SSH records StartShellEvent and EndShellEvent — **DONE** | 1h | t001 |
| [t003](done/t003.md) | The web shell records StartShellEvent and EndShellEvent with the browser's user agent — **DONE** | 45m | t001 |
| [t004](done/t004.md) | REST, GraphQL, CSV and the dashboard render the shell events — **DONE** | 45m | t002, t003 |
| [t005](t005.md) | Production check: an SSH and a web shell session each leave a start and an end row | 30m | t004 |
| [t010](t010.md) | Native SSH records the client's address: the :22 edge carries PROXY protocol from the load balancer | 1h | t005 |
| [t006](done/t006.md) | Render parity — **DONE** | 30m | t005 |
| [t007](done/t007.md) | Simplify — **DONE** | 20m | t006 |
| [t008](done/t008.md) | Test coverage — **DONE** | 30m | t006 |
| [t009](t009.md) | Closeout | 15m | t007, t008, t010 |

## Definition of done

- On production, as the QA user on a paid service in a Pro workspace, `bex ssh srv-…` and the dashboard Shell each produce, within two minutes, a `StartShellEvent` row when the session opens and an `EndShellEvent` row when it closes, on REST `GET /v1/owners/{ownerId}/audit-logs`, GraphQL and the CSV export, with `metadata.service` (the `srv-` id), `metadata.instance`, `metadata.source_ip` and, for the web shell, `metadata.user_agent`.
- A session the gateway refuses after identifying the caller records `StartShellEvent` with `status: error`; an unidentified connection records nothing.
- No terminal content, command or environment reaches either row (ADR035's content-free boundary, tested).
- Agent-session (`ags-…`) SSH records neither event.
- ADR035 records the gateway's audit write path and its residual.

## Progress (2026-11-16)

- **Implemented:** the ssh-gateway writes `shell.StartShell`/`shell.EndShell` audit rows (`sshgateway.RecordShellEvent`) for native SSH (`nativessh`) and the web shell (`webshell`, with the browser's user agent), ids derived from the `ssn-` id and phase, fail-open. Migration 0289 adds `audit_events.instance_id` and skips the two shell verbs in the per-service event index trigger. REST maps both verbs, plus the denied `apps.ResolveSSHSession`/`apps.CreateShellSession` rows, to `StartShellEvent`/`EndShellEvent` with `metadata.service`/`instance`/`source_ip`/`user_agent`, and SSH gateway identities answer `actor.type: user`. GraphQL `instance`, the dashboard Resource cell ("Instance …") and the CSV metadata carry the instance. Agent-session SSH records neither.
- **t001 decision:** option A. The gateway role already had `INSERT` only on `audit_events`. `TestGatewayScopedRoleAllowsOwnSurfaceDeniesTheRest` now proves the role can insert a shell row (twice, recorded once) and cannot select, update or delete one. The real-DB run found that the index trigger reads `apps`/`tenants`, which the role is denied, so every shell row would have been lost. Migration 0289 fixes it. ADR035 § Audit contract records the write path and its residual.
- **Deviations:** (1) the dashboard has no per-event label table (it shows Render's event names as-is, as for every event), so t004 step 2 became the "Instance …" Resource line in all 8 locales; (2) a refusal is the resolver's existing denied row (mapped to `StartShellEvent`, now with the client address), not a second gateway row. The uncached re-check and the protected-environment admin gate record no row, as for every bex verb (ADR035). t006's cross-surface checks (REST, GraphQL and CSV agree; MCP has no audit tool) ran in tests (`internal/audit/shell_test.go`, the dashboard audit tests). The live comparison is part of t005.
- **Remaining:** t005, the production walk as the QA user once the pin carries the commit, then t009.
- **2026-11-18, `/pm unblock w9/m451` (verified evidence, `/loopx w9`):** the t005 release gate cleared. The newest pin on `origin/main`, `0070f86ab` (`chore(deploy): pin platform images to 054f29128f64`), carries the milestone commit `27f3a1a70`: `git merge-base --is-ancestor 27f3a1a70 054f29128f64` succeeds. The production cluster runs that pin: both `bex-api` pods (started 08:17Z) and both `bex-ssh-gateway` pods (08:18Z) report `bex-operator@sha256:dbfad3d6…` and the `dashboard` pod `bex-dashboard@sha256:66eae953…`, the digests `0070f86ab` writes. No decision was needed and none is recorded. The milestone returns to `w9/m451/`; t005 runs the DoD's live checks next, then t009.

## Production check (2026-11-18, t005)

Pin `0070f86ab` (`054f29128f64`) carries `27f3a1a70`. Both `bex-api` pods and both `bex-ssh-gateway` pods ran `bex-operator@sha256:dbfad3d6…`, the `dashboard` pod `bex-dashboard@sha256:66eae953…`. The walk ran as the QA user (`scripts/qa-login.sh`) in `bex-canary` (`tea-daif693dqjvc73e7as3g`, Scale plan, which includes Pro's audit logs). The fixture was web service `m451-11180908` (`srv-dbumm9seoepc73dlktlg`, `busybox:1.36.1` running `httpd`, starter plan, one instance `srv-dbumm9seoepc73dlktlg-jqf35096k5o6t75kuvvj`). A throwaway SSH key (`ssk-dbumnhseoepc73dlktng`) and a throwaway workspace API key (`02298ead-…`) were created for the walk. The gateway's host key matched the installed `bex-ssh-host-key` Secret (`SHA256:cwBcvu8o…`). The client's public address, as Cloudflare saw it from curl and from the browser alike, was `2600:1700:45f0:963f:cdcc:e455:8a7a:8823`. Playwright MCP did not connect, so a headless Chromium (playwright-core 1.63) drove the dashboard.

| Check | What was done | Observed |
| --- | --- | --- |
| Native SSH start and end rows | `bex ssh srv-dbumm9seoepc73dlktlg --output text -- 'echo …; sleep 5'` from the tree's CLI (`lego/cli`, Render CLI `4fd34231b637`), 09:11:50Z–09:11:59Z | `StartShellEvent` `aud-b1qqdvdvthua2su0c3lg` (09:11:53Z) and `EndShellEvent` `aud-71mo88j2lb34llb1f48r` (09:11:59Z), both `success`, actor `{type: user, id: usr-d9a5qasbt3bc73fiphn0, email}`, metadata `service`, `instance` and `source_ip`, no `user_agent`. **`source_ip` is `10.10.0.7`, the Hetzner load balancer's private address, not the client's.** See t010. |
| Web shell start and end rows | Service → Shell, wait for **Connected**, type a command, then `exit` (09:14:23Z–09:14:28Z) | `StartShellEvent` `aud-163b91i5arrjijhueu4k` (09:14:23Z) and `EndShellEvent` `aud-soij06ct66gi9645pjkp` (09:14:28Z), both `success`, the same actor, metadata `service`, `instance`, `source_ip` equal to the client address above and `user_agent` equal to the browser's `navigator.userAgent` (`… HeadlessChrome/153.0.8010.12 …`). |
| GraphQL agrees | `auditLogs(ownerId, startTime)` | The same four ids, `action` `StartShellEvent`/`EndShellEvent`, `actorId`, `actorEmail`, `sourceIp`, `userAgent` (web shell only), `instance` and `resource` `service:srv-dbumm9seoepc73dlktlg`, as on REST. |
| CSV export | Workspace Settings → Audit Log → **Export as CSV** for 2026-11-18 (149 rows) | The four rows, `actor` the QA user's email, metadata keys `instance`, `service`, `source_ip` (plus `user_agent` on the web shell rows), with the same values as REST. |
| An unidentified connection records nothing | `ssh` with the key before it was registered (09:11:29Z): `Permission denied (publickey)` | No row between the fixture's `CreateServerEvent` and the API key's creation (09:11:35Z). |
| No terminal content | Both sessions ran a marker command | Neither the REST rows nor the CSV contain the commands or their output. |

## Source + Goal linkage

- **Source:** `/pm-brainstorm one aspect of render parity to w9` (2026-11-16), filed by `/pmx them all to w9`. Evidence:
  - [render.com/docs/audit-logs](https://render.com/docs/audit-logs) (read 2026-11-16): `StartShellEvent` (SSH session started) and `EndShellEvent` (SSH session ended), from the CLI or the service's Shell page; shell metadata "can include `service`, `instance`, `source_ip`, and `user_agent`". Render's changelog added `EndShellEvent` on 2026-03-25. Both names are in the pinned enum.
  - bex: native SSH (`lego/backend/internal/sshgateway/nativessh/server.go`) and the web shell (`lego/backend/internal/sshgateway/webshell/websocket.go`) write only `ssh_sessions` (`lego/backend/internal/store/sshsessions.go`), which no tenant surface reads.
  - [w4/m321](../../w4/done/m321/README.md) t006 asked to "Decide `StartShellEvent` and `EndShellEvent` for SSH and the web shell (ADR035) … Record a decision or a follow-up for each"; its result records neither.
- **Goal linkage:** [ADR008 § Render parity first](../../../docs/ADR008-vision.md#render-parity-first); a workspace admin can see who opened a shell on a production instance, as on Render.
- **Expected outcome:** shell access, the strongest human access path into a running instance, is visible in the workspace audit log and its export.
- **Why now:** w4/m321's required decision was skipped, and shell access has no tenant-visible audit at all; depends on w9/m450's `source_ip`/`user_agent` columns (t001 depends on `w9/m450/t002`).
- **Decision state:** all `/pm-brainstorm` **recommendations**, not user decisions: (1) the gateway write path is an INSERT-only grant on `audit_events` (t001; alternative: merge `ssh_sessions` at read time); (2) native SSH carries no user agent (Render: "when available"); (3) a refused session from an identified caller records `status: error`; (4) agent-session SSH is a cloud-agent surface whose reference product is Devin, so it records neither Render event.
- **Render parity task:** included. REST, GraphQL, the CSV export and the dashboard labels change; MCP has no audit-log tool on Render or bex.
