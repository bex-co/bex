# CLI sweep 946 — webhook create discipline (live, 2026-10-20)

One disposable webhook created (and deleted, 204) plus refused probes. No new bug filed.

- `http://` receiver at create → the same coded `WEBHOOK_URL_INVALID` as PATCH (sweep 945) — HTTPS-only is enforced identically on both write verbs.
- A duplicated `eventFilter` entry (`deploy_ended` twice) → 201 accepted; the stored filter is a set semantically (one delivery per event, sweep 943), so the duplicate is harmless. Recorded without filing: no contract found that requires rejecting duplicate filter entries.
