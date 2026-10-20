# CLI sweep 634 — lone JSON surrogates degrade to U+FFFD on both content channels

**Verdict: no bug filed.** Same isolated QA session, `bex-canary`, CLI at `c8717afdf`, 2026-10-19. Owned fixture `qa634-web` (`srv-dbbb2oivfpec73dm0i9g`, free busybox), deleted at close: REST 404, zero `qa634` names. Raw-body REST probes (the values are not secrets).

- **Lone surrogate in an env var:** `PUT` with the raw JSON value `"a\ud800b"` → `200`, stored and read back as `a�b` — the unpaired surrogate became U+FFFD at decode. No 400, no truncation, no wedged revision (the var updated normally afterward).
- **Same on secret files:** content `"x\ud800y"` → `201`, read back `x�y`. Both content channels share the standard Go `encoding/json` replacement behavior.
- **Paired surrogates are lossless:** `"\ud83d\ude00"` stored and revealed as the actual 😀 character.
- This matches Go's documented decoder semantics; a stricter API could refuse, but silent replacement of an unpaired surrogate is the ecosystem-standard outcome, not a bex divergence.

## Unrun

- Lone surrogates in names/keys (grammar-refused before content decoding, per sweeps 581/625).
