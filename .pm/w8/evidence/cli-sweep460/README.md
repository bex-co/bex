# Native redeploy variants + multi-resource logs — sweep460

Passing functional coverage, no new issue; one board-maintenance event. Same isolated session as sweeps 454–459, `bex-canary`, HEAD checkout build (`69e56eb30`; no `lego/cli` delta), 2026-10-18T04:38–04:53Z.

**Board collision resolved:** the shipped concurrent hunt claimed `w8/m114` (environment-group metadata) while this loop's uncommitted edge-blocked-logs milestone held the same number. The pull/stash cycle surfaced it as a workstream-README conflict; this loop's milestone was extracted intact from the stash, renumbered to [w8/m115](../../m115/README.md) (all `id:` frontmatter and cross-links updated, sweep458 evidence repointed), and the README conflict resolved keeping both entries. Nothing was lost; upstream `m114` and this loop's `m115` now coexist.

Coverage: multi-resource request logs (`-r <a>,<b> --text <marker>`) return each service's planted line exactly once; `--maintenance-mode` on a free web service refuses by name (paid-plan 400, no state change); `--auto-deploy` toggles `yes/commit ↔ no/off` truthfully in both spellings; native `deploys create --clear-cache --wait` rebuilds to `live` (81 s) and serves HTTP 200; Key Value allowlist set/readback/clear round-trips (`--clear-ip-allow-list` → `[]`).

Cleanup: exact-owned `srv-dba52qa1lfsc73chdci0`, `srv-dba52qi1lfsc73chdck0`, `red-dba55ar4pcls73bt1sc0` deleted by ID (404 / named not-found verified); baseline byte-identical. No plan mutations. Session remains active; the loop continues from sweep461.
