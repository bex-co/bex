# w3 · m506 — A running instance keeps at most 6,000 application log lines a minute, as on Render, so one flooding service cannot delay or drop every tenant's logs

**Worker:** worker3 **Goal:** the log shipper drops an instance's application lines beyond Render's documented 6,000 a minute, counts what it drops, and Loki's ingestion limits are explicit, so one service printing in a loop costs only its own excess lines. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](t001.md) | Measure each instance's line rate on production | 30m | — |
| [t002](t002.md) | Alloy: cap `app_logs` at 6,000 lines a minute per pod | 45m | t001 |
| [t003](t003.md) | The live tail applies the same cap | 1h | t002 |
| [t004](t004.md) | Explicit Loki ingestion limits and alerts on dropped or refused lines | 45m | t001 |
| [t005](t005.md) | dev-3 flood drill | 45m | t002, t003, t004 |
| [t006](t006.md) | Ship and verify on production | 30m | t005 |
| [t007](t007.md) | ADR010 and ADR018 | 20m | t006 |
| [t008](t008.md) | Render parity | 20m | t007 |
| [t009](t009.md) | Simplify | 15m | t008 |
| [t010](t010.md) | Test coverage | 30m | t008 |
| [t011](t011.md) | Closeout | 10m | t009, t010 |

## Definition of done

- `app_logs` drops each instance's lines beyond 6,000 a minute and counts them; request and datastore logs are not capped.
- The live tail drops the same excess.
- On dev-3 a flood of 20,000 lines a minute keeps at most the cap and a neighbor loses nothing.
- Loki's ingestion limits are explicit; alerts fire on 429s and sustained drops; production shows no 429 retries after the pin.

## Source + Goal linkage

- **Source:** `/pm-brainstorm` for w3 2026-11-18/19 (topic: one aspect of Render parity, project and environment observability and the log explorer), filed by `/pmx them all to w3` 2026-11-19 after a read-only capture of Render's live project Observability views in the user's Render workspace (`docs/render-artifacts/project-observability.md`).
- **Goal linkage:** ADR008 § Render parity first: [render.com/docs/logging](https://render.com/docs/logging) documents the limit. bex has no cap on tenant application logs; only sandbox pods are limited.
- **Recommendation, not a user decision:** the token bucket (100/s, burst 6,000) is the recommendation, since Render does not document its window; ADR010 records where it differs from a per-minute count.
- **Expected outcome:** one service printing in a loop loses its own excess, as on Render, instead of slowing or dropping every tenant's logs.
- **Why now:** Loki runs single-tenant with default ingestion limits, so a flood from any one service gets every push refused with 429 and retried, delaying or dropping everyone's lines. The sandbox pipeline already proved the mechanism.
- **Render parity included:** the capped history is what every log surface reads.
