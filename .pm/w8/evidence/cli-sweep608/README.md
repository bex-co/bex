# CLI sweep 608 — pin-wide audit for the w8/084 raw-response pattern: exactly one defective call site

**Verdict: no new filing; the defect class has exactly one member, already tracked.** Offline audit of the pinned module (`render-oss/cli@v1.1.3-0.20260909214233-a764810a7682`), 2026-10-19. No production calls, no fixtures. Follows up sweep 607's Unrun item.

- **Method:** every `client.ErrorFromResponse(resp)` call site in `pkg/`+`cmd/` was traced to how `resp` is produced. Sites fed by `…WithResponse` calls receive parsed structs whose `JSON4xx` fields the checker inspects — those are sound. The defect needs a plain generated call returning raw `*http.Response`, on which the checker's reflection finds no error field and returns nil.
- **Result:** exactly one such site exists in the pin — `pkg/postgres/repo.go:91` (`RestartPostgres`), the w8/084 defect re-verified live in sweep 607. Every other repo either uses `WithResponse`, checks `resp.StatusCode` itself (`pkg/storage/repo.go` presigned up/download: `StatusCode < 200 || >= 300` → error), or inspects the response for its streaming handshake (`pkg/sandbox`, `pkg/logs`).
- **Consequence:** no second false-success verb is hiding in the pinned client at this pin. On the next pin bump, re-running this grep pair (`ErrorFromResponse` sites × non-`WithResponse` assignments) is the cheap regression check; w8/084's upstream patch remains the fix path.

## Unrun

- The same pattern in the interactive TUI views that bypass repos (spot-checked via the repo trace; views call repos for mutations).
- Re-audit at the next CLI pin.
