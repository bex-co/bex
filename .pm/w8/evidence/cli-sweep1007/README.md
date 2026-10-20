# CLI sweep 1007 — env-var value fidelity (live, 2026-10-20)

One disposable free web service created and deleted (204). Values compared programmatically, never printed. No new bug filed.

- Batch PUT → 200; read-back is byte-identical for a value containing a newline, a value containing `=` signs, and an empty value. No trimming, escaping, or empty-value rejection.
