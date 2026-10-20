# CLI sweep 826 — CLI Key Value create/get/delete cycle (live, 2026-10-20)

Disposable free Key Value instance created and deleted entirely through the pinned CLI; REST confirms absence (coded 404). No new bug filed.

- `bex keyvalues create --name qa826-kv --plan free --confirm -o json` → `{"data":{…}}` with `red-…` id, `version: "8"`, status `creating`.
- `keyvalues get <id>` returns the same object; `keyvalues delete <id> --confirm` succeeds and the REST GET afterwards is 404 — CLI and REST agree on the lifecycle.
