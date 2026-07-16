# w10 · m3 — Ledger & board truth sweep round 2 (ADR018 backlog + FUTURE-MAYBE sync)

**Worker:** worker10 **Goal:** The parity loop's instruments stop lying: ADR018's gap-backlog table (five rows claiming open/partial/blocked whose owners are all resolved in `done/`), three doc-pointer gaps, and two `FUTURE-MAYBE.md` trigger-state drifts are corrected — so the next brainstorm round doesn't re-litigate resolved gaps the way round 12's miners chased stale docs. Coordinates with — never duplicates — `w5/013` (three ADR018 table cells) and w10/m2 (ADR006/render-artifacts). **Status:** done — 2026-07-15

**Resolution (2026-07-15):** every drift verified against its `done/` artifact before editing. ADR018: manual-scaling/`w2/008`/`w8/m13` backlog rows flipped to done with pointers (the `w4/016` and Key Value `red-` rows had already been corrected by their own closeouts); events row now cites `w3/m7` + `w3/m16` (m16 done — its remaining ◐ is deliberate divergence, not open work); PR-previews row and the "untracked (low)" row both rewritten to cite the DO_NOT_DO rejection + per-item owners (registry-credential Docker-build remainder → open `w6/m34`). ADR028:131 and the CLI-checklist `login` row were already fixed by `w7/m35`/`w4/m27` closeouts — verified, no edit. FUTURE-MAYBE: `w8/m7` pricing entry moved to Done (payments half stays deferred); delivery-worker clause now names double-delivery (`worker.go:55`); Traefik entry was already in Done. `w7/done/m36` bookkeeping synced (statuses/rows/Resolution pointer at `0a782eff` + ADR022); w4's m23 checkbox was already fixed by `w4/021`. Prettier clean.

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- | --- |
| t001 | Fix the five stale ADR018 gap-backlog rows with resolution pointers | 30m | — | — **DONE** |
| t002 | Doc pointers: ADR028→w7/m35, ADR018 events row→w3/m16, PR-previews wording | 20m | — | — **DONE** |
| t003 | FUTURE-MAYBE sync: w1/m41 promotion note, w8/m7 → Done, delivery-dedup clause | 20m | — | — **DONE** |
| t007 | Fix two more stale pointers: CLI-checklist login row → w4/m27; ADR018:214 cells | 20m | — | — **DONE** |
| t008 | Sync stale done-work bookkeeping: w7/done/m36 task files + w4 README m23 checkbox | 20m | — | — **DONE** |
| t004 | Simplify | 10m | t001, t002, t003, t007, t008 | — **DONE** |
| t005 | Test coverage | 10m | t004 | — **DONE** |
| t006 | Closeout | 15m | t005 | — **DONE** |

## Definition of done

Every ADR018 gap-backlog row's status matches the board (each of the five stale rows carries its resolution + pointer, verified against `done/`); ADR028's open finding cites w7/m35; ADR018's events row cites w3/m16 (not done w3/m7); the PR-previews row's "untracked (low)" wording cites the DO_NOT_DO user rejection instead of implying a backlog omission; FUTURE-MAYBE entry 2 notes its promotion to w1/m41 and the w8/m7 pricing entry sits in Done; the multi-replica entry names webhook delivery-worker double-delivery; prettier clean.

## Source + Goal linkage

- **Source:** `/pm-brainstorm` round 13, 2026-07-15 — parity-ledger mining + board recon: stale rows ADR018:198 (`w5/004` resolved, manual-scale row all-✅), :217 (`w2/008` retired with both bugs re-verified), :218 (`w8/m13` shipped), :221 (`w4/016` flipped ◐→✅), :222 (owner pointer should be open `w9/m6`, not retired `w1/021`); pointer gaps ADR028:131 (no m35 cite) and ADR018:35/193 (events row cites done w3/m7 while open w3/m16 owns the ◐); FUTURE-MAYBE items 2 and 4 out of sync with the board; the delivery-dedup clause is the round-13 borderline finding (`.pm/w3/done/m11/done/t003.md:41`) folded into the existing multi-replica trigger entry rather than a new item.
- **Goal linkage:** the ledger is the parity loop's instrument — round 12 already demonstrated stale docs produce false roadmap findings (that lesson created w10/m2; this is the ADR018/board-file counterpart).
- **Expected outcome:** the next `/pm-brainstorm` round's miners read accurate status from ADR018, ADR028, and FUTURE-MAYBE.
- **Why now:** three independent miners tripped over this staleness this round; each future round pays the cost again until it's fixed.
- **Render parity:** omitted — docs/board-only, no REST/GraphQL/MCP/UI surface change.
