# CLI sweep 809 — datastore list scoping + KV path (live, 2026-10-20)

Read-only probes; nothing created. No new bug filed.

- First-pass anomalies dissolved under a second route: `GET /postgres?name=<canary-baseline>` and `GET /services?name=hello-go` returned `[]` without `ownerId`, and hit immediately with `ownerId=tea-daif693dqjvc73e7as3g`. Unscoped REST lists default to the caller's default workspace (a different one of this account's three workspaces — its services/Postgres answered the unfiltered calls), which is the documented behavior (ADR018 w4/m128: omitting `ownerId` keeps default-workspace behavior). The CLI wrapper pins `BEX_WORKSPACE` to canary, which is why CLI lists never showed this.
- Key Value's REST base is `/v1/key-value` (`lego/backend/internal/keyvalue/rest.go:177`); `/keyvalue` and `/keyvalues` → 404. Canary KV list → `200 []`.
- Synthetic `dpg-…` GET → coded 404 `NOT_FOUND`.
