# CLI sweep 516 — `bex docs` destination

**Verdict: no bug filed.** Same isolated session, HEAD build `8e57d4eaa`, 2026-10-19T09:20Z. Local launcher surface only.

| probe | observed result |
| --- | --- |
| `bex docs` with a PATH-shimmed `open` | Opens exactly `https://github.com/bex-co/bex/blob/main/docs/bex-cli.md` — the branding layer's `DocsURL` override (`lego/cli/internal/branding/branding.go:34`), not `render.com/docs`. Exit `0`, no stdout chat. |
| Destination liveness | The URL answers `200`. |
| `docs --help` | Bex-branded ("Open the Bex docs in your browser"). |
| `$BROWSER` handling | Ignored by the upstream opener (`pkg/dashboard/dashboard.go`: darwin `open`, linux `xdg-open`, windows `rundll32`) — identical under the Render CLI, so the BROWSER=/usr/bin/true guard other sweeps use does **not** neutralize `docs`; the shim capture above is the safe way to probe it. Recorded as harness knowledge, not a bex defect (the one unshimmed run opened a real browser tab on this machine — harmless, read-only page). |

## Safety and unrun checks

- Nothing created or mutated server-side; the shim ran only in this run's private PATH and is discarded with its temp dir.
- Unrun: `docs` on Linux/Windows openers (same upstream code path, different binary name), `bex upgrade` (deliberately not run — a self-update would replace the tested binary mid-loop).
- Deploy lag does not apply (local binary surface).
