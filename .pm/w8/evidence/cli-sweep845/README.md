# CLI sweep 845 — CLI `logs` command surface (live, 2026-10-20)

Read-only CLI probes on the baseline (read only). No new bug filed.

- `bex logs -r srv-daif… --limit 3` prints the baseline's real request log lines (the sweep-803 REST contract through the CLI). The required flag is `--resources` (sweep 803's `--service` guess was the pinned CLI's different spelling, as its help shows).
- `--level bogus845` → client-side enum refusal listing the eight levels; synthetic resource → `no resource found with ID or name …` — both clean named errors before any streaming starts. Log streaming itself (`--tail`) was not held open; the history path is what this sweep exercises.
