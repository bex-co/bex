# w5 · m122 — Tenant logs: classify lines by source, not by message text

**Worker:** worker5 **Goal:** Platform, kubelet and CNPG-internal lines never reach tenant logs in any pipeline, because classification uses where a line came from, and one fixture set is shared by the shipper and the Go tests. **Status:** done

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Close w8/050's remaining gaps — **DONE** | 45m | — |
| t002 | One fixture file for the shipper test and the Go tests — **DONE** | 45m | t001 |
| t003 | Read tenant pod logs from files, as build logs already do — **DONE** | 1h30m | t002 |
| t004 | CNPG: keep Postgres records, drop other JSON — **DONE** | 30m | t002 |
| t005 | Render parity — **DONE** | 15m | t003, t004 |
| t006 | Simplify — **DONE** | 15m | t005 |
| t007 | Test coverage — **DONE** | 30m | t005, t006 |
| t008 | Closeout — **DONE** | 10m | t007 |

## Definition of done

- Live and history reads for apps, Postgres and Key Value show no kubelet "logs gone" lines and no CNPG `net/http` TLS-handshake lines.
- Message-text kubelet filters are gone (source-based classification); any that remain have a recorded reason and identical coverage in every pipeline, enforced by the shared fixtures.
- `scripts/test_log_shipper.py` and the Go log tests read the same fixture file.

## Source + Goal linkage

- **Source:** Last-24h code review, 2026-10-05, of w8/050 (`9ccb92920`) and w8/047 (`c41d1aa91`). Both are deny-lists that grow one QA finding at a time, and the per-pipeline lists have already drifted: Postgres history lacks one kubelet drop, Key Value history lacks both, and Go direct reads lack the CNPG drop.
- **Goal linkage:** ADR010 observability; ADR043 tenant isolation (no platform internals in tenant views).
- **Expected outcome:** Tenant logs contain only tenant output.
- **Why now:** Two deny-list fixes in one day, and the lists already disagree between pipelines.
- **Render parity included:** tenant log contents change on every surface.
- **Boundary:** the shipper config is platform GitOps (`deploy/gitops/base/log-shipper.yaml`); validate changes with the GitOps render/validate scripts.

## Evidence — 2026-10-06

**w8/050's remaining gaps closed (t001).** The kubelet's "logs gone" answers and CNPG's `net/http` TLS-handshake lines no longer reach tenant reads in any pipeline:

- Postgres and Key Value history: both pipelines read the containers' files (t003), so the kubelet's answers never enter, and the CNPG allow-list (t004) drops the `net/http` JSON.
- Postgres and Key Value direct reads, through the logs service and the adapters' own fallback alike, go through one `datastorelogs.ParseLine`: an untimestamped line is dropped as the kubelet's, and `CNPGLine` drops the `net/http` JSON (it used to keep logger-less JSON). The two paths used to differ: the logs service turned the kubelet's answer into a platform notice, and Key Value's own reader had no kubelet handling at all.
- App reads were already file-sourced in history (w4/m174) and keep the platform placeholder in direct reads.

**One table (t002).** `lego/backend/internal/logs/testdata/tenant-log-lines.json` holds 21 Postgres cases, 2 Key Value cases and the kubelet's two bodies. `scripts/test_log_shipper.py` runs the Postgres and Key Value cases through the real `database_logs` and `keyvalue_logs` pipelines in the chart's Alloy image. The Go tests run them through `CNPGLine`, the datastore fallback reader and the App direct read. Changing either side alone fails a test:

- the Go allow-list keeping logger-less JSON fails `TestCNPGLineFollowsTheSharedTable`;
- the shipper keeping the manager's JSON fails `test_postgres_follows_the_shared_table`.

**Classified by source (t003).**

- Shipper: `database_pods` and `keyvalue_pods` moved from `loki.source.kubernetes` to `loki.source.file` + `stage.cri`, as `app_pods` did in w4/m174, with the same `/var/log/pods/*<uid>/<container>/*.log` glob and the `filename`/`stream` labels dropped, so the stream labels are unchanged. The last message-text kubelet drop (`kubelet_pod_log_dir_gone`) is gone. `test_tenant_pipelines_read_only_container_files` asserts that the rendered config feeds `app_logs`, `database_logs` and `keyvalue_logs` from file sources only; pointing `database_pods` back at the API tailer fails it.
- Go direct reads: with `Timestamps: true` (both pod-log readers) every container line carries the kubelet's stamp, so a line without one is the kubelet's own answer, whatever it says. `kubeletLogsGonePrefixes` is deleted. An App read re-types any untimestamped line as the platform placeholder (`TestTheKubeletsOwnAnswerIsAPlatformLine`, including an answer the old prefixes didn't know); a datastore read drops it (`TestCollectDropsAnUnstampedLine`, `TestCollectDropsTheKubeletsOwnAnswers`, `TestManagedPostgresPodLogsUnwrapCNPG`). A follow is the one exception: it starts at its since second, and the kubelet stamps nothing on the rest of a split line whose first chunk it skipped as older, so a follow holds an unstamped first line. That line is the kubelet's answer when the stream ends there and the tenant's text when more output follows (`TestFollowHoldsAnUnstampedFirstLine`).
- Node coverage is unchanged: Alloy discovers only its own node's pods (w3/m13), for the API source and the file source alike.
- `scripts/gitops-validate.sh` passes.

**CNPG allow-list (t004).** One rule on both sides: a `logger=postgres`, `msg=record` entry whose `record` is a JSON object is rewritten to PostgreSQL's stderr shape with its severity as `level`; a line that is not a JSON object passes verbatim (plain text, a JSON array, invalid JSON); every other JSON object is the instance manager's and is dropped. The shipper's four match stages became one drop stage and the rewrite. Both read the line as Alloy's JSON stage does: exact keys, any value types, and an omitted record field rendered as nothing (CNPG omits empty fields; the shipper's template now defaults each with `or`). The table covers instance-manager JSON, probes, the `net/http` line, non-record Postgres JSON, a non-object `record`, a record without a message, an empty record, a numeric process id, an object `detail`, a non-string `logger`, capitalised keys, a bare `null`, each severity's level, detail and hint, and plain text.

**Render parity (t005).** REST, GraphQL, MCP, the CLI and the dashboard read the same two sources (Loki history and the direct reads), so tenant log contents change identically on every surface. Render's logs show a service's or database's own output only; bex now holds the same line by source. Two behavior changes are recorded in ADR010: JSON in a Postgres container that is not a PostgreSQL record is dropped (it is always the instance manager's), and a direct datastore read drops an untimestamped line.

**`/simplify` (t006)**, three reviews. Applied:

- The quality review showed the "one table, no drift" claim did not yet hold. A CNPG record without a message, which CNPG emits because it omits empty fields, rendered as `<no value>` in the shipper and as nothing in Go. Go also read JSON unlike Alloy: case-insensitive keys, non-string values treated as plain text, numbers as `1.048576e+06`, and objects as Go maps. `CNPGLine` now decodes into a map with exact keys and formats values as Alloy does. The templates default absent fields with `or`, and seven new table rows pin these cases on both sides.
- CI never ran the shipper harness when only the table changed. The fixture is now in both of `gitops.yml`'s path lists.
- The follow exception above: a split line straddling the follow's since second would have shown the "logs gone" notice in place of the tenant's text.
- The reuse review found the two datastore direct reads disagreeing on the kubelet's answer. `core.SplitPodLogLine` is the one timestamp split, and `datastorelogs.ParseLine` is the one datastore line rule, which the logs service now calls too. The CNPG unwrap and level labeling are no longer written twice.
- The efficiency review measured `stage.cri` rebuilding a split line at a cost that grows with its square: a 50 MiB line took 5.7 s and 176 GiB of allocation, holding up every pipeline on the node, and Loki drops anything past 256 KiB anyway. All four `stage.cri` blocks now cap reassembly at 200000 bytes and truncate the rest.
- The Key Value stream labels were unguarded: removing their `filename`/`stream` drop left the harness green. Both datastore tests now assert the full label set. `gitops-validate.sh`'s w4/m174 guard now covers the app, Postgres and Key Value pipelines (file source, capped `stage.cri`, the label drop), and fails on either regression. The harness's source check also covers `build_logs`.
- `cnpg_json` is the literal `` `true` ``: `@ != null` only worked because no object has a `null` key.
- Comments and docs: the shipper header (the API source now feeds only the platform and Traefik pipelines; the `/var/log` mount serves every tenant pipeline), ADR010 (a stale "placeholders below" reference; the rollout's one-time `LogDeliveryLokiRejected`/`LogDeliveryEntriesDropped` warnings), the datastorelogs package doc, the Loki terminator comment, and `Query.within`.

Skipped:

- Alloy `declare` to share the file-tailing blocks: it would change each source's component path and orphan its file positions, so every node would re-read every file once.
- `stage.cri` exact paths in place of the `*<uid>` glob: about 330 lstat calls a second, not worth diverging from the existing pipelines.
- One shared Go loader for the table: each package decodes a different part, as with w5/m118's vector files.
- A harness flake: `PlatformLogsTest`'s relabel path, which this change does not touch, missed one marker once in five runs.

**Tests (t007).** New: `TestCNPGLineFollowsTheSharedTable`, `TestCollectDropsTheKubeletsOwnAnswers`, `TestCollectDropsAnUnstampedLine` (inverting `TestCollectKeepsUnstampedLine`), `TestTheKubeletsOwnAnswerIsAPlatformLine` (replacing the two prefix tests), `TestFollowHoldsAnUnstampedFirstLine`, a kubelet body in `TestManagedPostgresPodLogsUnwrapCNPG`, and in the harness `test_postgres_follows_the_shared_table`, `test_keyvalue_follows_the_shared_table` and `test_tenant_pipelines_read_only_container_files`. A build-tail fake gained the timestamp every real kubelet line carries. Each fix fails its test when reverted (10 mutations):

- Go keeping logger-less JSON, decoding case-insensitively, or formatting with `fmt.Sprint` fails the table test;
- the shipper keeping the manager's JSON, or its templates without `or` defaults, fails the harness's table test;
- `database_pods` back on the API tailer fails the harness's source check and `gitops-validate.sh`;
- the App rule matching only known kubelet words fails the platform-line test;
- the datastore fallback keeping unstamped lines fails its two tests;
- the follow without the hold fails its test;
- the logs service's datastore branch on the old placeholder path fails the Postgres direct-read test.

**Gates.**

- `scripts/test_log_shipper.py`: 7 tests pass in the chart's pinned Alloy image (Docker).
- `scripts/gitops-validate.sh` passes.
- The backend suite on fresh Postgres, OpenFGA and OpenBao is green (68 packages); `/ship`'s gate runs it again before the push.
- `make lint`: 0 issues in all four modules, including the whole-program dead-code pass.
