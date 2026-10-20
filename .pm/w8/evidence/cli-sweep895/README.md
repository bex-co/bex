# CLI sweep 895 — env-var `revision` is read-only observational (live, 2026-10-20)

Disposable free web service created, exercised, deleted (204, then GET 404). No new bug filed.

- Per-key GET exposes `revision: "evr1_…"` (the coherent-map CAS token), but sending `revision` back on a PUT → 400 `unknown field "revision"` for both a stale and the current token — the write body is strictly `{value | generateValue | from…}`-shaped, and the value never moved (`v1` throughout the refused PUTs). The revision informs readers; it is not a writable precondition field on this REST surface.
