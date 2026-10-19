# CLI sweep 577 — REST env-var routes: replace semantics to the process (owned fixture)

**Verdict: no bug filed.** Same isolated QA session, `bex-canary`, HEAD-built CLI at `77ec448ae`, production pin `d98d07f62`, 2026-10-19T20:20–20:28Z. Owned fixture `qa577-env` (`srv-dbb7oh7o17is73furtig`, free `ealen/echo-server`, whose response reports the process environment) via same-credential REST (`/tmp/qa501/env577.py`); deleted with service + env-vars + URL 404 proof. Closes sweep 524's unrun residue.

- **Single-key PUTs** all echo `{key, value}` and reach the process after the deploy each write opens: update-in-place, new key with `a=b=c` in the value, a value carrying a newline and unicode (`riga1\nriga2-caffè-€`, byte-identical in the process report), explicit empty value (present, not dropped), lowercase key. Refusals named: `1BAD` and a 254-char key (253 cap), missing/non-string `value` at the schema boundary, and `PORT` with the reserved-key sentence pointing at the port field instead (w2/m95).
- **Bulk PUT is a true replace:** writing `[QA577_ONLY]` removed every prior key from the process after its deploy settled (`{QA577_ONLY, PORT}` only). Single DELETE → 204; repeat DELETE and GET of a deleted/never-existed key → `404 env var not found`.
- **Two cells investigated and cleared, recorded so they are not re-litigated:**
  - `RENDER_EXTERNAL_URL` is accepted and reaches the process verbatim. bex injects no `RENDER_*` platform variables (the only operator-injected env is the literal `PORT`, the one reserved key), so there is no platform value to shadow — it is an ordinary user variable with a suggestive name.
  - One read immediately after a bulk PUT (issued mid-burst, with six single-key deploys queued) returned the pre-bulk set. Re-measured cleanly, the same read converges in 0.6 s. Recorded as a one-off under burst load, not reproduced; if it recurs under load it deserves its own note with timing data.

## Unrun

- Secret-file routes (sweep 456 era) and env-group linking through these writes.
- Value size cap (no refusal found at ordinary sizes; a dedicated >64 KiB probe was not run).
