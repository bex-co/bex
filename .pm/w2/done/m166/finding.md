# Clearing datastore IP rules opens external access while the CLI says it is blocked

- **Severity:** major. A network-disable operation leaves the authenticated external path open. Credentials are still required; no unauthenticated data access or compromise was tested or claimed.
- **Attribution:** backend access-intent compatibility plus operator enforcement semantics, not a launcher defect. The pinned CLI sends the correct empty array, and its text output explicitly promises external denial.
- **Versions/context:** 2026-10-02 ~08:11–08:25 UTC, plus pooler corroboration ~08:30–08:39 UTC; production `https://api.bex.co/v1/`; human QA device OAuth; workspace `tea-d98210cbbpdc73dcrkvg`; installed `/opt/homebrew/bin/bex` v0.2.1; Render v2.27.0 pin `a764810a768202704e7206eb7b87a47211fcd98e`. Unmodified same-pin Render against Bex also returned PONG. Server deployed SHA unknown; initial diagnosis source `1287ee5bf`, pooler follow-up source `a9407d21d`. The PG launcher control used an isolated checkout build `dev-8c5bc` to avoid the already-filed v0.2.1 CA-provisioning release gap.
- **Modes:** mutations/discovery non-TTY JSON or text; `kv-cli` in a bounded PTY with installed redis-cli; PG diagnostic client used verified TLS, a private server-CA file, forced IPv4, and saved fixture credentials passed only in environment variables.

## Expected contract and conflicting history

The exact pin's [pkg/text/ipallowlist.go:17–20](https://github.com/render-oss/cli/blob/a764810a768202704e7206eb7b87a47211fcd98e/pkg/text/ipallowlist.go) emits `IP allow-list: empty (external connections blocked)` for both datastores. Its [Key Value update builder](https://github.com/render-oss/cli/blob/a764810a768202704e7206eb7b87a47211fcd98e/pkg/keyvalue/update.go) lines 101–104 and [Postgres builder](https://github.com/render-oss/cli/blob/a764810a768202704e7206eb7b87a47211fcd98e/pkg/postgres/update.go) lines 73–75 encode the clear flag as a present empty `ipAllowList` array. Source was inspected in the downloaded module at this exact pin.

Render's [Key Value external-access documentation](https://render.com/docs/key-value#enabling-external-connections) and [Postgres access documentation](https://render.com/docs/postgresql-creating-connecting#restricting-external-access), checked 2026-10-02, both specify that the clear flag disables external access. Their internal paths remain available. This is datastore semantics; do not generalize it to web-service HTTP lists.

Bex deliberately implements the opposite: `docs/ADR021-keyvalue-management.md:37,47` and `keyvalue/service.go:787–793` say clear opens all source IPs and attribute that meaning to Render. w4/m116 chose sticky publication on this premise; w7/m5 likewise intentionally removed enforcement for empty rules. Those records are contrary evidence that must be reconciled explicitly, not ignored. The new fact here is the pinned-client expectation plus live admitted traffic after the purported disable. This is an established compatibility error, not a claimed recent regression.

## Owned fixtures and reproduction

All resources were newly created in verified free capacity, absent from the workspace baseline, and recorded with creation responses. Use fresh names/IDs on retest; these historical fixtures are deleted.

```sh
bex keyvalues create --name qa-20261002-c4e13d-kv --plan free --region oregon --memory-policy cache --confirm -o json
# Returned red-davmed6de41s73canqqg, initially private.
bex keyvalues update red-davmed6de41s73canqqg --ip-allow-list cidr=<QA_SOURCE_IPV4_CIDR>,description=QA-current-source --ip-allow-list cidr=<QA_SOURCE_IPV6_CIDR>,description=QA-current-source --confirm -o json
bex kv-cli red-davmed6de41s73canqqg -o interactive -- PING
bex keyvalues update red-davmed6de41s73canqqg --name qa-20261002-c4e13d-ren --confirm -o json
bex keyvalues update qa-20261002-c4e13d-ren --clear-ip-allow-list --confirm -o json
bex keyvalues get red-davmed6de41s73canqqg -o text
bex kv-cli red-davmed6de41s73canqqg -o interactive -- PING
# Denial control, then exact clear-body diagnostic replay below.
bex keyvalues update red-davmed6de41s73canqqg --ip-allow-list cidr=198.51.100.0/24,description=QA-denial-control --confirm -o json
bex kv-cli red-davmed6de41s73canqqg -o interactive -- PING

bex postgres create --name qa-20261002-c4e13d-pg --plan free --version 18 --disk-size-gb 1 --region oregon --ip-allow-list cidr=<QA_SOURCE_IPV4_CIDR>,description=QA-current-source --ip-allow-list cidr=<QA_SOURCE_IPV6_CIDR>,description=QA-current-source --confirm -o json
# Returned dpg-davmgtmde41s73canr5g. Wait for available.
bex postgres update dpg-davmgtmde41s73canr5g --ip-allow-list cidr=198.51.100.0/24,description=QA-denial-control --confirm -o json
bex postgres update dpg-davmgtmde41s73canr5g --clear-ip-allow-list --confirm -o json
bex psql dpg-davmgtmde41s73canr5g --command 'SELECT 42 AS qa_probe;' -o json -- -X
```

Source CIDRs above are redacted; use the actual routed client addresses. `198.51.100.0/24` is a nonmatching documentation range, not another tenant's source.

| Observation | Exit / elapsed | Result |
| --- | --- | --- |
| KV available after initial publish | 0 / 1.826s | CLI PING returned `PONG` |
| Installed KV clear | 0 / 1.212s | JSON `data.ipAllowList: []` |
| Installed KV get, text | 0 | `IP allow-list: empty (external connections blocked)` |
| KV PING after clear | 0 / 1.678s | `PONG`, no timeout |
| Fresh KV retry at 08:15:53, over 80s after clear began | 0 / 1.668s | `PONG`; status `available`, empty list |
| Nonmatching KV CIDR | bounded at 15.817s | Connect error, no PONG; termination exit 0 is a harness interrupt, not success |
| Exact empty PATCH after denied control | HTTP 200 | `public:true`, `ipAllowList:[]` |
| Unmodified pinned Render `kv-cli ... -- PING` after that PATCH | 0 / 1.675s | `PONG` |
| PG allowlisted, checkout Bex `psql` | 0 / 3.668s | Query row `42` |
| Installed PG clear | 0 / 2.388s | JSON `data.ipAllowList: []` |
| Checkout Bex `psql` after clear | 1 / 3.559s | Local error: `IP address (<QA_SOURCE_IP>) not in allow list for qa-20261002-c4e13d-pg` |
| Direct PG client using saved credentials after clear | 0 / 2.006s | `42` over verify-full TLS |
| Direct PG client after nonmatching-rule control | 2 / 0.713s | TLS connection closed, no row |
| Second installed PG clear | 0 / 1.146s | Empty list again |
| Direct PG query after second CLI clear | 0 / 1.371s | `42` over verify-full TLS |

**The PG contradiction is deliberate evidence.** Pinned `pkg/tui/views/psql.go:58–66` performs a local allowlist check before launching psql; empty entries fail that check. That local guard is working. It does not enforce the server network policy for applications or existing credentials. A separate direct protocol probe bypassed only the CLI convenience check, using the same legitimate fixture credentials and full TLS verification. Do not claim `bex psql` itself connected after clear.

The direct diagnostic command was `psql -X -A -t -c 'SELECT 42 AS qa_probe;'`, with `PGHOST`, `PGHOSTADDR` (IPv4), `PGPORT`, `PGDATABASE`, `PGUSER`, `PGPASSWORD`, `PGSSLMODE=verify-full`, `PGSSLROOTCERT=<private CA file>`, and `PGCONNECT_TIMEOUT=6` provided privately. Obtain those values through the authenticated connection-info response for the owned fixture. No password, URI, token, or CA capture is included here. It was a read-only query against an empty disposable database.

### PgBouncer route corroboration

A fresh free Postgres 18 fixture, `dpg-davmnas5o9vs73dt7phg` (`qa-20261002-75f48a-pool`), was created at 08:30:35 UTC with `--connection-pool pgbouncer`, 1 GB disk, and both routed source CIDRs. After status became `available`, primary and pooled verified-TLS queries each returned `42` (exit 0, 1.471s / 1.498s). Installed `bex postgres update dpg-davmnas5o9vs73dt7phg --clear-ip-allow-list --confirm -o json` exited 0 in 0.653s; readback retained `connectionPool:pgbouncer` and returned `ipAllowList:[]`. Fresh direct queries at 08:37 UTC, over two minutes after the clear, still returned `42` through both endpoints (exit 0, 1.463s / 1.258s). Authenticated detail returned `available`, `public:true`, `poolerEnabled:true`, and an empty list. This confirms the same defect on the pooler route; it is not a separate finding.

The control restored only the QA source CIDRs, then ran `bex postgres update dpg-davmnas5o9vs73dt7phg --connection-pool none --confirm -o json` (exit 0, 0.678s). Connection-info omitted pooled fields, the saved pool endpoint rejected a query (exit 2, 0.674s, TLS EOF), and the primary still returned `42` (exit 0, 1.580s). Pool disable therefore worked independently of the allowlist-clear defect. Both probes used the same private-credential, forced-IPv4, verify-full method above.

## Exact wire evidence

Authenticated diagnostic replays, not in-flight CLI traces. The empty payloads are byte-equivalent JSON to the pinned builders; no `public` field was added. The actual CLI User-Agent was used (`render-cli/2.27.0 (macOS - <OS_VERSION>)`). Authorization is redacted, and the owner email is replaced by `<QA_EMAIL>`; all other response fields/types/envelopes are retained.

```http
PATCH /v1/key-value/red-davmed6de41s73canqqg
Authorization: Bearer <QA_HUMAN_ACCESS_TOKEN>
Content-Type: application/json

{"ipAllowList":[]}

HTTP 200
Date: Fri, 02 Oct 2026 08:18:37 GMT
Content-Type: application/json
CF-RAY: a442470679fa2510-SJC

{"id":"red-davmed6de41s73canqqg","name":"qa-20261002-c4e13d-ren","plan":"free","status":"config_restart","suspended":"not_suspended","createdAt":"2026-10-02T08:11:32Z","updatedAt":"2026-10-02T08:18:37.327785166Z","owner":{"id":"tea-d98210cbbpdc73dcrkvg","name":"bex","email":"<QA_EMAIL>","type":"team"},"region":"fsn1","dashboardUrl":"https://dashboard.bex.co/r/red-davmed6de41s73canqqg","version":"8","options":{"maxmemoryPolicy":"allkeys_lru","persistenceMode":"journal_snapshot"},"ipAllowList":[],"externalHost":"red-davmed6de41s73canqqg.kv.bex.co","public":true}
```

```http
PATCH /v1/postgres/dpg-davmgtmde41s73canr5g
Authorization: Bearer <QA_HUMAN_ACCESS_TOKEN>
Content-Type: application/json

{"ipAllowList":[]}

HTTP 200
Date: Fri, 02 Oct 2026 08:21:38 GMT
Content-Type: application/json
CF-RAY: a4424b72c97c0dfd-SJC

{"id":"dpg-davmgtmde41s73canr5g","name":"qa-20261002-c4e13d-pg","plan":"free","version":"18","status":"available","databaseName":"dpg_davmgtmde41s73canr5g","databaseUser":"dpg_davmgtmde41s73canr5g_user","diskSizeGB":1,"diskAutoscalingEnabled":false,"highAvailabilityEnabled":false,"readReplicas":[],"suspended":"not_suspended","createdAt":"2026-10-02T08:16:54Z","updatedAt":"2026-10-02T08:21:38.499740268Z","region":"fsn1","dashboardUrl":"https://dashboard.bex.co/d/dpg-davmgtmde41s73canr5g","externalHost":"dpg-davmgtmde41s73canr5g.db.bex.co","public":true,"ipAllowList":[],"poolerEnabled":false,"connectionPool":"none","backupsEnabled":false,"ownerId":"tea-d98210cbbpdc73dcrkvg","owner":{"id":"tea-d98210cbbpdc73dcrkvg","name":"bex","email":"<QA_EMAIL>","type":"team"}}
```

## Root cause and bounded repair

Paths below are repo-relative and line references are at diagnosis HEAD `1287ee5bf`.

- KV: `lego/backend/internal/keyvalue/rest.go:358–374` preserves a non-nil empty list; `service.go:838–849` clears entries but only sets `Spec.Public=true` for a nonempty list. Clearing never withdraws or otherwise denies the public route. `SetIPAllowList` at `service.go:685–689` delegates to that same patch.
- PG: `lego/backend/internal/postgres/rest.go:457–474` passes the empty list to `PostgresPatch`; `service.go:1048–1055` copies it but leaves `Spec.Public` alone. The dedicated setter at `access.go:61–71` separately writes only the entries, so repairing PATCH alone would miss its callers.
- Data plane: `lego/operator/internal/sniproxy/allowlist.go:55–58` returns true for an empty layer. KV route construction/authorization (`cmd/kv-sni-proxy/main.go:90–119,148`) keeps public routes and checks the empty resource and environment layers. PG does the same at `cmd/pg-sni-proxy/main.go:135–159,248`, including its primary/pooler/replica hostname variants.
- This is not stale readback or auth failure: the valid source control works, the nonmatching-rule control denies, and subsequent empty writes admit fresh connections. Runtime results match current code. Password authentication remains in place.

**Target:** a present empty datastore allowlist is an explicit external-disable intent; omitted means unchanged; nonempty restores listed-source access; explicit Bex controls have a documented precedence. Implement this at the shared datastore access-intent boundary. One bounded option is to couple explicit clear to external publication/denial state (and rule-addition to re-enabling) while preserving `public` overrides intentionally; settle the full matrix before choosing that representation. Ensure all setters use it, including the currently separate PG setter. Do not hide the mismatch by changing upstream text, suppressing connection information, or relying on a client-side check.

Do not blindly flip the shared `AllowedBy(empty)` default: it also makes absent environment layers composable and supports existing CR contracts. Preserve pre-existing public/empty resources unless a new explicit operation changes them, or provide a reviewed compatibility migration. Correct the false Render claim in ADR021, MCP descriptions and datastore editor labels. Keep service/static HTTP empty-list semantics and inherited environment deny-all unchanged. Internal connections must stay intact regardless of the external rule.

## Counted blast radius and acceptance

Eight logical API writers: each of KV and PG has REST PATCH, dedicated REST PUT `/ip-allow-list`, GraphQL `setKeyValueIpAllowList` / `setDatabaseIpAllowList`, and MCP `update_key_value` / `update_postgres`. KV's writers converge on one patch; PG has two mutation sinks (general patch and dedicated setter). Read/preview counterparts and create defaults need explicit matrix coverage. CLI aliases are `keyvalues`/`kv`/`keyvalue` and `postgres`/`pg`; no new flags or fork are needed.

Two proxy callers invoke `ParseAllowList`; two `AllowedBy` expressions each evaluate resource AND environment. PG's shared route entry serves primary, pooler and read replicas. Dashboard has two datastore consumers of the shared `IPAllowListEditor`: `features/keyvalue/components/key-value-networking-panel.tsx` and the database access-control panel, through their GraphQL hooks. The editor also serves HTTP services, so change datastore labels/intent without globally changing the shared component's meaning. MCP input descriptions currently explicitly say empty opens all IPs (`keyvalue/mcp.go:87,92,171`, `postgres/mcp.go:317`).

Regression matrix: omitted / explicit empty / nonmatching / matching v4 and v6 / malformed / explicit Bex public true and false; preview vs write; new vs existing public/private resources; protected and unauthorized writes; internal path retained; environment AND composition; primary/pooler/replica; all eight API writers. Live acceptance must observe actual packets/queries after convergence and include reuse of pre-clear valid credentials. Readback-only tests miss this bug.

## Dedupe and limits

- Searched open/done `.pm` for clear flag, empty/open/deny semantics, and external-disable phrases. w7/m45 and w9/m4 assert field round-trip, not external denial. w5/065 likewise records empty-array success. None owns the admitted-traffic contradiction.
- w4/m116/t002 intentionally implemented sticky publication and tested it. This filing corrects its stated Render premise and extends coverage; keep its successful private-to-public behavior. Its historical record must remain intelligible.
- w1/m150 and w4/m117 cover HTTP allowlists; their empty-is-open requirement is intentionally out of this fix. w9/m164 concerns dual-stack denial diagnostics, not empty-list admission. Forced IPv4 and a direct deny/allow pair isolate the PG issue from that known trap.
- Targeted history: `6208c7500` introduced the explicit sticky-publish policy; `92387494f` extracted the shared empty-is-open predicate. No later repair found on main through `1287ee5bf`; deployed revision remains unknown.
- Current anti-goals reviewed. This is supported hosting parity, not a new security policy for unrelated resources, a CLI fork, or paid-fixture work.
- Not live-tested: internal connection preservation, PG read replicas, every GraphQL/MCP writer, dashboard interaction, cross-workspace denial, or all legacy-state combinations. These are counted acceptance obligations, not claims of coverage. The primary and PgBouncer external routes were live-tested.

## Cleanup

Cleanup completed for the renamed KV `red-davmed6de41s73canqqg`, original PG `dpg-davmgtmde41s73canr5g`, and pooled PG follow-up `dpg-davmnas5o9vs73dt7phg`: all CLI deletions exited 0, authenticated detail requests returned 404, and IDs were absent from the corresponding CLI lists. Subsequent direct PG queries failed with no row, including both saved primary and pool URLs for the follow-up; the former KV TLS endpoint closed the handshake (`SSLEOFError`). No shared or pre-existing resources were mutated.
