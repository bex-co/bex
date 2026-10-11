# w4 · m199 — Preserve the first timestamp of a reassembled CRI log record

**Worker:** worker4 **Goal:** a below-cap logical container record has the same first-chunk timestamp in durable history and live tail, so overlap renders once without merging distinct records. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Deliver and pin a collector that retains the first partial timestamp | 90m | — |
| t002 | Verify the four CRI pipelines and shared log readers | 45m | t001 |
| t003 | Render parity | 20m | t002 |
| t004 | Simplify | 15m | t003 |
| t005 | Test coverage | 35m | t003 |
| t006 | Closeout | 20m | t004, t005 |

## Definition of done

Repeat the actual a177 probes in [finding.md](finding.md), using a fresh owned Free Python image web fixture and its quiet stdout-only command:

- One emitted 50,075-byte message written in two pieces 120 ms apart has equal complete message bytes, instance and **first-chunk raw timestamp** in bounded GraphQL/REST history and native SSE. The message stays below the existing 64 KiB cap and retains its original case and end markers.
- A fresh 1440×1000 history-only page shows one matching row; enabling Live still shows **one full row**. A freshly reloaded 390×844 Live page shows one. The 8,071-byte delayed-write and 47-byte short controls stay complete with matching timestamps and one row.
- The locked collector image, rendered with the production App pipeline and fed the exact local P/F fixture, emits the reassembled record at **10:35:00.100000001Z**, not the final 10:35:00.220000002Z. The single-F control remains at 10:35:00.300000003Z. Record image/source identity and complete replay output.
- Delete the owned service; REST lookup and its public URL return 404 and inventory contains no owned fixture. Revoke only the verification session, verify API 401 and zero browser cookies, and remove its local handles.

Other resource/pipeline classes and the original m96 cron/framing/cancel acceptance controls were not exercised in a177. They are explicit required verification work in t002/t005/t006, not claims of live coverage here. The independent over-limit message-size mismatch remains in w4/256.

## Source + Goal linkage

- **Source:** continuous functional `$qa-find-bugs w4`, a177, 2026-10-11, [complete finding/probes](finding.md). Follows a176's timestamp observation with an independently reproduced below-cap case.
- **Goal linkage:** ADR008 reliable hosting and ADR010 one shared log core: logs must faithfully identify emitted records while users diagnose a service.
- **Expected outcome:** the demonstrated record uses its first CRI chunk's timestamp across the historical reader and pod live tail and renders once on overlap; smaller controls remain exact.
- **Why now:** app, Postgres, Key Value and build pipelines share the same pinned CRI stage, and the confirmed timestamp-only duplicate survives w4/256's separate message-cap fix.
- **Scope:** collector partial reassembly/artifact pin, shared-source verification and docs. Keep the pod-based live tail, Loki storage/retention, existing caps, precise cursors and message-sensitive frontend key. Old historical records age out unchanged.
- **Dedupe:** regression/follow-up to w4/done/m96's record-identity guarantee. Open w4/blocked/m174 owns loss of a five-line fast-exit set, not this complete record's first/final timestamp mismatch; keep its residual open. w9/done/053's millisecond precision-format normalization remains correct.
- **Render parity:** included because REST/GraphQL/MCP/UI receive the corrected historical timestamp. Public Render documents generated timestamps and timestamp paging; exact CRI behavior was not authenticated against Render.
- **Unverified:** production kubelet version; live Postgres/KV/build/private/worker/cron versions of the case; original m96 scheduled/manual/cancel/framing variants; a fixed collector artifact. The current pinned image was actually replayed, and the web duplicate was actually repeated from fresh desktop/mobile pages.
