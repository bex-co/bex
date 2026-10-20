# CLI sweep 625 — env var `generateValue` and key grammar at the REST boundary

**Verdict: no bug filed.** Same isolated QA session, `bex-canary`, CLI at `c8717afdf`, 2026-10-19. Owned fixture `qa625-web` (`srv-dbbap25u6sus73bpjdmg`, free busybox), deleted at close: REST 404, zero `qa625` names. Generated values were only ever observed inside the probe; none are recorded here.

- **`generateValue` generates and reveals to the owner:** `PUT [{key, generateValue: true}]` stored a 44-character base64 value (32 random bytes), and the per-key GET reveals it — the owner can read back what the server minted. Re-PUTting the same per-key `generateValue` **rotates**: a fresh value replaced the old one, with the revision bumped. Sending both `value` and `generateValue` in one entry → `400 invalid request body at /0`.
- **Key grammar is named:** lowercase keys are accepted (`gen_lower`); leading digit, dash, and space keys each fail with `400 (ENVIRONMENT_VARIABLE_INVALID): invalid environment variable name "…" — use letters, digits and underscores, not starting with a digit, at most 253 characters`. Grammar refusals were atomic — the stored set was untouched after each.
- **Bulk replace confirmed again from this angle:** a later bulk PUT of a single unrelated key replaced the whole set (the generated key disappeared from the list) — the sweep-577 replace semantics, so per-key PUT is the surgical verb.

## Unrun

- Whether the generated value reaches a running process unchanged (needs a live app reading it; the shape matches plain vars per sweep 577's marker proof).
- `generateValue` inside an env group.
