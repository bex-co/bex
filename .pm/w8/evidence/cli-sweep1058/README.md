# CLI sweep 1058 — env-var key charset and batch-replace semantics (live, 2026-10-20)

One disposable free web service created and deleted (204). No new bug filed.

- Key charset, one PUT per key: digit-leading (`9QA…`), dash (`QA-…`), and dot (`QA.…`) keys → 400; underscore-leading (`_QA…`) and lowercase keys → 200 (lowercase acceptance matches the earlier verbatim-lowercase observation; no pinned uppercase-only contract exists).
- The batch PUT is a replace-all: sequential single-key PUTs leave only the last accepted set stored. Per-key writes belong to the single-key endpoint (sweep 991).
