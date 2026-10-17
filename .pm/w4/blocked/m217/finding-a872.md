# a872 — original live cases pass; identified millisecond-boundary records still collide

Why: an authoritative record ID must survive regardless of how many fractional digits its timestamp needs; a successful replay of the original burst does not prove that broader m217 requirement.

**Classification:** source-verified acceptance residual in existing w4/m217, not a newly observed production boundary failure. Functional QA 2026-10-17. Research main `f8d2279fbc3a388d40d3e4dfa9554c1f9ef15dd7`; the last mapping change is `37944bca4`. No product fix is implemented here. [Complete native requests/responses, source probe, literal inputs/outputs, source hashes and cleanup](evidence/a872-acceptance-and-boundary.json). Original a751 evidence remains unchanged.

## What actually passed live

The owned Free `qa-20261017-a872-records` / `srv-db9jmp3ahr9s73ftjd5g`, sole deploy `dep-db9jmp3ahr9s73ftjd60`, used the exact original a751 Node 24 Alpine command. One GET `/emit` after Live returned200 / `QA_A751 ready\n` at **09:10:20.395Z**. No second emission or fixture mutation was used.

- Fresh desktop 1440×1000 FAST history returned20 original records,20 distinct IDs and20 distinct timestamps, `hasMore:false`, and mounted all20 row indices at **09:10:43.260Z**. They occupy three millisecond buckets of1,8,11 records; the old approximate key would produce3, while the current reader correctly produces20. Do not claim this new burst placed every record in one bucket.
- Desktop SLOW and UNIQUE each returned/rendered20 at **09:11:08.834Z / 09:11:11.864Z**. A fresh390×844 FAST page returned/rendered20 at **09:12:56.508Z**, document width390. Mobile SLOW and UNIQUE also returned/rendered20 at **09:13:17.510Z / 09:13:20.510Z**.
- Mobile FAST and SLOW native SSE200 replay, then pause, each stayed20 before/live/paused at **09:12:59.547Z / 09:14:26.767Z**, never40. This is ordinary replay; no connection fault was forced.
- Complete authenticated REST reads returned20 distinct IDs for each control with completed coverage. Each ID, original timestamp and message equals its matching native GraphQL record. No raw messages or timestamps were rewritten.

These observations satisfy the original fixture's count/replay cases. They do not prove the README's separate distinct-ID guarantee for a genuinely exact-millisecond timestamp next to a finer timestamp.

## Deterministic source counterexample

The actual unmodified `dashboard/src/features/logs/lib/map.ts` was bundled with the existing esbuild executable and imported by Node. Its SHA256 and the complete repeatable build command/script are retained in the evidence. The fine entry is the captured native FAST record at `2026-10-17T09:10:19.863264187Z`. A second input was explicitly constructed with timestamp `2026-10-17T09:10:19.863Z` and the corresponding different backend-format ID, preserving the same instance and message.

These are two distinct instants and two distinct nonempty IDs. Both parse to millisecond1792228219863. `dedupeLogLines([fine, coarse])`, the reversed order and `mergeLogLines([fine], [coarse])` each return **1**, expected **2**. The control using the same authoritative ID with a coarser displayed stamp returns1 as required; the untouched20-record native batch still returns20. The constructed neighbor was never emitted or returned by the hosted fixture, and no intercepted/synthetic API response was used.

`map.ts:41–51` classifies identity by a regular expression for four or more fractional digits. At exactly three digits, `lineKey` discards a nonempty ID and returns the approximate bucket key. `LogIdentity.admit:158–171` then pairs the first coarse and fine bucket neighbors, without comparing their known IDs. Once mapping dropped that ID, callers cannot recover the distinction. The live fixture's timestamps all have sub-millisecond digits, so its passing result is fully consistent with this failure.

## Consumer trace rejects an unnecessary backend change

`lego/backend/internal/logs/loki.go:655` formats the original integer timestamp with UTC RFC3339Nano. [Go documents that this format omits trailing fractional zeros](https://pkg.go.dev/time#RFC3339Nano); a genuine exact-millisecond instant therefore legitimately arrives with three digits.

Production Kubernetes reports **v1.34.9**. Its [actual CRI log writer source](https://raw.githubusercontent.com/kubernetes/cri-client/v0.34.9/pkg/logs/logs.go) writes fixed nine-digit prefixes, but the consumer already handles that representation: `lego/backend/internal/core/base.go:78–88`, `SplitPodLogLine`, parses the prefix and returns UTC RFC3339Nano. `logs/service.go:1671–1683` uses that result. Thus live and Loki already supply canonical stamps to `render.go:81–88`, `logID`, for the same instant/message/instance. The initial raw-kubelet-format concern was rejected after reading this consumer. **Do not schedule backend timestamp or ID canonicalization work for this residual.**

## Bounded repair and compatibility target

Use every nonempty authoritative ID as the row identity, including exact-second/millisecond timestamps. Distinct IDs remain distinct even when message, instance and parsed millisecond match. The same ID remains one row when its displayed timestamp representation changes. Approximate bucket reconciliation is only a documented fallback when identity is absent; it must not pair two known different identities merely because one stamp is shorter.

Keep the real w9/053 ID-less precision-drift control and w6/m93 resume behavior. The newer test that fabricates two different nonempty IDs by changing an instant from nonzero nanoseconds to whole milliseconds must not override the distinct-ID contract: test same-ID changed-display-time and actual canonical producer inputs separately. Retain the original control instead of deleting it; explicitly model which inputs lack authoritative identity and why the fallback applies.

Five production dedupe calls remain: `use-log-history.ts:162,171`, `use-older-log-pages.ts:169`, `use-deploy-logs.ts:262`, and `map.ts:229` in `mergeLogLines`. The live ring separately admits/forgets through `LogIdentity` (`use-live-logs.ts:240–258`); the virtualizer uses the mapped key. The repair applies globally to this shared mapping/identity layer. Audit history heads, retained/older pages, live replay/eviction and deploy merges with both input orders. Service LogViewer covers web/static/cron/worker/private, both service/static deploy aliases use the deploy reader, and DatastoreLogViewer covers Postgres/Key Value. Only this web fixture was live-tested.

Keep LogEntry embedded in the shared CSR/SSR Apollo cache (`keyFields:false`), raw fields, ordering, literal/raw/display matching, bounded rendering and the outer SSE timestamp resume cursor. Missing/malformed stamps and ID-less entries keep an explicit fallback; there is no new API field, transport, error taxonomy or authentication behavior. Pending reader/reset behavior remains the existing behavior.

## Dedupe, remaining gate and limits

Searched the9675 board Markdown files for exact-millisecond/aligned/coarse identity and logID canonicalization, recent mapping history and the original owner's DoD. This is the existing m217 distinct-ID obligation, not another milestone or a fresh claim that the original a751 burst still fails. w4/256 owns message-terminator/cap canonicalization, which remains intact; m192 owns ordering/pause, w9/053 and m93 own the recorded replay controls. No matching separate owner or later main fix was found.

DoD audit: bullet1's native/UI count case passes; bullet2's20/20 controls and ordinary replay pass, but its distinct-ID requirement fails the constructed module probe. Bullets3–4 need renewed identity/consumer tests after the bounded repair. Bullet5 cleanup passes. Add t008 before the standing closing tasks; keep t004 pending for post-repair parity/live replay, reopen t005/t006 for the changed implementation, and leave t007 pending. The milestone is actionable in w4, not waiting only for a deployment.

[Current Render documentation](https://render.com/docs/logging) describes runtime search/filtering, live tail and individual deploy logs. It does not specify this identity edge case; no authenticated Render comparison is claimed. Backend identity/raw contracts and m217's existing target establish the requirement.

**Unverified:** production occurrence or frequency of the constructed boundary pair; live datastore/sibling families, forced disconnect, saturated paging, mixed backend versions and malformed stamps. Backend/dashboard image digests and served asset URLs are retained; an independent image-to-Git-SHA mapping is not established. A focused `yarn test` invocation could not run because the vitest executable is absent (exit127); that is a harness setup limit, not a failing product assertion. Earlier implementation suite results retain their original date. No screenshot is presented as proof of the constructed failure.

**Cleanup:** native DeleteService returnedtrue at **09:15:54.030Z**. Service/sole deploy404 and all seven baseline ID families matched at **09:17:23.737Z**. Exact session logout exited0, whoami401, cookies0, one blank page and removed owned handles at **09:17:29.984Z**. Exact App/Deployment/serving/build/publish/purge/predeploy selections were all empty at **09:17:30.641632Z**; public URL404 is retained in the evidence headers. No paid operation, foreign mutation or product-code change occurred.
