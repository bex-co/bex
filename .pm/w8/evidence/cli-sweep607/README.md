# CLI sweep 607 — `restart` cross-type dialects; the `dpg-` false success re-verified (already tracked as w8/084)

**Verdict: no new filing — one known upstream defect re-verified live, everything else correct.** Same isolated QA session, `bex-canary`, CLI at `1a49cb806`, 2026-10-19. Owned fixtures `qa607-kv` (`red-dbba3gkgnrlc73bhssn0`) and `qa607-web` (`srv-dbba3gqbpdss73ci0nkg`), both deleted (404 / not-found, zero `qa607` names).

- **The false success, reproduced cleanly:** `bex restart dpg-00000000000000000000 --confirm` and `dpg-11111111111111111111` both print `"<id> restarted successfully"`, exit 0 — while the identical REST call `POST /v1/postgres/{id}/restart` returns `404 NOT_FOUND`. Root cause is the pinned client, exactly as the existing audit records: `pkg/postgres/repo.go:91` passes the raw `*http.Response` from generated `RestartPostgres` to `client.ErrorFromResponse`, whose reflection finds no error field on that type, so any HTTP status reads as success (the `084` upstream-prep audit in this workstream's evidence). This is **w8/084, already blocked on upstream submission** — not refiled. The bex server is correct here; no server fix exists.
- **Everything else in the verb is honest:** Key Value ids (creating or available) → `key / value stores cannot be restarted` (client-side, before any request; the instance stayed `available`); garbage id → `unknown resource type`; bare `srv-` → server `404`; restart of a web service during its first deploy → `restarted successfully` (accepted and executed); the sweep-606 suspended/static refusals stand.

## Unrun

- Restart of a real owned Postgres (the free slot is held by the baseline; never restarted per fixture rules).
- The same raw-response pattern in other pinned-client verbs (084's audit scopes it; a fresh sweep could re-grep the pin for other `ErrorFromResponse(*http.Response)` call sites).
