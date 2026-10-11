# m192 verification — 2026-10-11 UTC

Shipped fix: `8a6d82e19` (deploy.yml run 38106148255 succeeded: all test jobs, build, deploy).

## Change

- `useLiveLogs` (`dashboard/src/features/logs/hooks/use-live-logs.ts`): `enabled` left the buffer identity. A user pause closes the EventSource and keeps rows already received. On resume the dedupe set keeps its epoch, so replayed rows stay single. A reader change still clears the buffer and drops a stale effect's pending batch. A reader change means resource/type/text/textMode/instance, or the new `readerKey`, which the viewer sets from the store-only filters via `storeOnlyFilterKey`. Status resets on pause/resume as before.
- `mergeLogLines` (`lib/map.ts`) dedupes first-copy-wins, then sorts stably by parsed time plus sub-millisecond digits, never by lexical RFC3339. Equal stamps keep history-then-arrival order. A stampless line stays beside its neighbor. The window lower bound is applied in the same pass, replacing the viewer's second `Date.parse` filter. Neither source array is mutated.
- `DeployDetailPage` keys `DeployLogPanel` by deploy id. The retained build tail therefore cannot carry rows into another deploy of the same service.

## Caller and family matrix (t003)

| Surface | Disposition |
| --- | --- |
| Service Logs (LogViewer; web/private/worker/cron share it) | Web verified live below; others share the identical component (source-only) |
| useDeployLogs (deploy detail, services + static routes) | Unit: followBuild→false keeps streamed build lines and closes the tail; remount per deploy id. Its own history/live sort is untouched (out of scope, see notes) |
| DatastoreLogViewer (Postgres, Key Value) | History-only; uses neither useLiveLogs nor mergeLogLines; unchanged |
| LogLineList (3 consumers) | Unchanged; key-based anchors survive reorders; 5,000 cap and virtualization unchanged |

## Automated gates

- New regressions failed against the old code: 4 merge-order tests and 2 pause-retention hook tests. Controls stay green: readerKey reset, a reader round trip, the stale pending batch, the viewer's DOM order and readerKey wiring, the build-finish retention test and the deploy remount test.
- `yarn lint` (typecheck + eslint + knip) passed. `yarn test` passed: 490 files, 4503 tests. `yarn build` passed.

## Live acceptance (fresh Free image fixture `srv-db5g3k8k3t2s73e5va2g`, busybox:1.37, Application logs / Last hour, 1440×1000)

- **Ordering:** on a fresh `live=0` page, enabling Live and reading the whole list gave START, n=1…159: 160 rows, zero backwards steps, zero duplicates.
- **Pause:** the history head was n=153. Live streamed n=154–158. After pausing, each of n=154–158 was present exactly once, the bottom ended at n=159 and the footer said "Live tail paused". Resume: list through n=165, no backwards steps, no duplicates.
- **API parity:** the GraphQL, REST and MCP `list_logs` probes for `QA_M192_TICK n=150` each returned HTTP 200 with the same message and timestamp `2026-10-11T03:31:09.332332447Z`. REST id is `…-973tnqhnkgdk89je4jos-2026-10-11T03:31:09.332332447Z-e91a99d2`, hasMore false. Envelopes are unchanged.
- **Redeploy tail:** with Logs open and Live on, Deploy latest image created `dep-db5g6dgk3t2s73e5va5g`, which became live. Without reload, the new instance `tmtp2` START landed in time order at row 180, interleaved with the old instance's final ticks. Per-instance sequences were monotonic. There was no disconnected banner and the footer showed streaming. The public endpoint returned 200 `QA_M192_HTTP_OK`.
- **Narrow mobile (390×844, paused):** 34 rows mounted (bounded DOM), no horizontal page overflow.
- **Cleanup:** the fixture DELETE returned 204. Service GET and the public host both returned 404. Service IDs are back to the 5-service baseline. This run's session was revoked (`qa-login.sh --logout` → ok) and the jar was removed. The only console errors were this run's own probes: one 400 create retry and the 404 after delete.

## Render parity (t004)

Render's logging docs cover search, ranges, Live tail and instance filtering. They do not specify pause retention, and authenticated Render pause behavior stays unmeasured. bex's SSE transport divergence (ADR010) is unchanged. No backend, cursor or API contract change was made, so no ADR edits were needed.

## Simplify (t005)

Three-angle review applied. The pause flag and reader reset now share one render-phase block (no duplicate key state). The store-only reader key is a `types.ts` helper. The comparator is shorter. The window bound is folded into the merge, so each line is parsed once. Skipped: storing parsed ms on `LogLine`, which would widen a shared type. Also skipped: unifying `useDeployLogs`'s `localeCompare` sort and `useLogHistory`'s ms sort with the new comparator, which the finding scoped out unless a sibling gap reproduces.

## Remaining limits

The store-only filter, refusal and disconnect races are covered by unit tests only. Non-web families and finished-build retention are source and unit only. The busy 5,000-line tail was not exercised live.
