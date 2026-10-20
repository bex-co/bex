# CLI sweep 619 — all three open notes (w8/132, 133, 134) now fixed on main; production still behind

**Verdict: deploy lag — notes stay open until live-verified.** Same isolated QA session, `bex-canary`, 2026-10-19. Checkout pulled to `c8717afdf`, QA CLI rebuilt, session re-verified. Read-only spot checks only.

- **Code state:** `ca30db3a3` landed the w8/134 fix (`connecting a Blueprint answers its classified fetch failure, not a bare 500`), joining `f8008fcb3` (132) and `f31004f7c` (133). Every researched finding this loop has open is now fixed in source.
- **Production state:** the newest pin commit (`c8717afdf`) pins platform images to `2f780251dd25`, which predates all three fixes. A spot check confirms it: `POST /blueprints/generate` with a malformed body still returns the bare 500.
- **Closure rule for these notes:** when a production pin passes `ca30db3a3`, re-run the sweep-595 logs matrix (expect named refusals for `task`/`taskRun`/`sandbox` on `/logs`, `/logs/values`, `/logs/subscribe`), the sweep-596 generate/connect probe sets (expect named 400s and classified 4xx/503 fetch failures), and only then close 132–134 via `/pm` with the live evidence.
