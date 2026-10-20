# CLI sweep 696 — early-access (`ea`) read-only recon (live, 2026-10-19)

Nothing created; list calls only.

- `bex ea sandboxes list -o json` → `null` (empty; same CLI nil-slice wart as sweep 656).
- `bex ea objects list` without a region → client `--region is required (or set RENDER_REGION environment variable)`; with `--region oregon` or `fsn1` → server 404 `not found` (Render's object-storage route is not a bex surface).
- `bex ea sandbox-groups list` → server 404.

The `ea` tree is upstream Render CLI surface that bex does not serve; refusals are clean 404s rather than false successes. Sandbox creation was not attempted (pillar-5 machinery, out of this loop's safe-fixture scope).

No bug filed.
