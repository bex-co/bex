# CLI sweep 1259 — restart, clear-cache deploy and instances on a static site (live, 2026-10-20)

One free no-build static site (`mdn/beginner-html-site`) created with `bex services create --type static_site`, live in 27 s, deleted (GET 404). No new bug filed.

- `bex restart <static>` → exit 0, `restarted successfully`, and a new deploy row (`bexTrigger: api`) republishes the same commit. REST `POST …/restart` answers 200. Render's restart docs give no static-site support matrix (“a special form of manual deploy … identical Git commit and configuration”), so this is consistent, not a divergence.
- `bex deploys create <static> --clear-cache --wait` → live in 24 s.
- `GET …/instances` and `bex services instances <static> -o json` → `[]` (static sites have no runtime instances), as ADR018's instances row documents.
