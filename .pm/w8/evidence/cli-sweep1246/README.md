# CLI sweep 1246 — rename, then reuse the old name (live, 2026-10-20)

One free echo-server service (`srv-dbbim26cbnls73997ff0`) created as `qa-20261020-61af2e-rn`, renamed with `bex services update <id> --name qa-20261020-61af2e-rn2`, then deleted (GET 404). No new bug filed.

- The rename changes only the display `name`. `immutableName`, `slug` and the URL stay `qa-20261020-61af2e-rn`, and the service keeps serving at its original URL (ADR049 identity contract).
- `bex services create --name qa-20261020-61af2e-rn …` afterwards → exit 1, `409 (CONFLICT): name "qa-20261020-61af2e-rn" is already in use`. This is deliberate: `errServiceNameInUse` reserves a name another service “was created as or is displayed as” (`apps/service.go:2571–2577`). Render allows duplicate names, so this is a recorded uniqueness divergence. `GET /v1/services?name=<old name>` returns the renamed holder, so the holder is discoverable. Not filed; the wording could name the holder, but that is polish.
- Because the old name is reserved while the holder exists, the w8/144 log/metric leak cannot be reached through a rename; it needs a delete.
