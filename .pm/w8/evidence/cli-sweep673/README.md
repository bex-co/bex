# CLI sweep 673 — service clone (`--from`) region failure (live, 2026-10-19)

Fixtures `qa673-src` (`srv-dbbbiolfsg2s73ako8i0`) and its clone deleted and verified absent (REST 404 ×2, zero `qa673` services).

- `bex services create --from <src>` without `--name` → client `name is required`. From a synthetic id → named resolution failure (`No service named '…' in workspace …`).
- With `--name qa673-clone` but no region override → `region must be one of: frankfurt, ohio, oregon, singapore, virginia`. Root cause: the source reads back `"region":"fsn1"` (bex's truthful installation region, ADR018) and the pinned CLI's clone path validates the inherited region against Render's enum. This is the already-documented `fsn1` mismatch inside blocked [w8/075](../../blocked/075.md) (its sweep 72/75 evidence covers clone region refusals) — re-confirmed for a plain same-type web→web clone, not refiled.
- With `--region oregon` the clone succeeded (and still landed on `fsn1` server-side, per the ADR018 normalization). Observation: the clone's env-var list was empty although the source carried `QA673_MARK=clone-me` — env vars are not carried by the CLI clone; recorded without an upstream-oracle comparison, not filed.

No new bug filed.
