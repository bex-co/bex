# CLI sweep 781 — `services update` has no env/secret flags (live, 2026-10-19)

Disposable free web service `qa781-web` (`srv-dbbct6a720pc73c0kngg`). Deleted and verified absent (REST 404).

`bex services update --env-var …` → `Error: unknown flag: --env-var` with usage; the update flag set carries no env-var or secret-file flag (env mutation rides REST PUT or blueprint sync). The service's env list stayed empty.

No bug filed.
