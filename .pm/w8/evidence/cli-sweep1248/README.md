# CLI sweep 1248 — static-site name reuse: delayed release and stale cached pages (live, 2026-10-20)

Two free static sites named `qa-20261020-4fc46e-stat` (`srv-dbbiur9q56vc73bn8ebg` from `mdn/beginner-html-site`, then `srv-dbbivuucbnls73997fs0` from `mdn/beginner-html-site-styled`), both no-build publishes created with `bex services create --type static_site … --publish-directory .`, both deleted (GET 404). The ledger audit shows no survivor.

- Unlike a web service (freed within 2 s in sweep 1244), a deleted static site's name stays reserved for about 100 s while it is torn down. `bex services create` answers `409 … name "…" is already in use` although no service with that name is listed. This is minor and not filed separately; the message could say the name is being released.
- **Finding → [w8/146](../../146.md):** the recreated site's first live deploy serves the deleted site's cached `index.html` (sha `e999a69c` instead of its own `5d04139b`) for over 6 minutes, until a second deploy moves to `rev-2`. Paths never requested on the old site serve correctly, so the bucket is clean and the cause is the static server's name/revision cache key.
