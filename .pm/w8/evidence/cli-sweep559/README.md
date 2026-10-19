# CLI sweep 559 — projects/environments reads and `services -e` filter dialects

**Verdict: no bug filed.** Same isolated QA session (`puncsky@gmail.com`), `bex-canary`, HEAD-built CLI at `77ec448ae`, production pin `d98d07f62`, 2026-10-19T13:18–13:19Z. Read-only throughout; baseline stayed 8 services.

- **`projects` list:** `bex projects -o json` and `-o yaml` both print `null`, exit 0 — the workspace has no projects. REST `GET /projects?ownerId=…` returns `200 []`, so the `null` is the pinned client's rendering of an empty page (same pattern as `workflows list`), not a server shape defect: the repo's `TestEmptyListRoutesSerializeAsJSONArray` gate (`lego/backend/internal/api/emptylist_test.go`, w4/m116/t006) holds every list route to `[]`, and `jobs list`'s REST twin also answered `[]` here.
- **`environments` selectors uniform:** a synthetic `prj-000…`, a service id, the workspace id, and a malformed `not-an-id` all return the identical `404 (NOT_FOUND): project not found`, exit 1 — cross-kind and malformed selectors neither enumerate nor split into a pattern 400.
- **`services -e` filter dialects** (closes sweep 511's deferred comma-multi leg): two bogus comma-joined ids, repeated `-e` flags, a service id used as an environment id, and `-e … --include-previews` all return empty (`null`, exit 0) — filter semantics, no lookup, no error. Empty filter spellings `-e ''` and `-e ,` drop to no constraint and return the full 8 rows, identical to unfiltered and to `--include-previews` alone (row counts verified: 8/8/8/8).

## Unrun

- A positive `-e` match against a real environment containing services: the workspace has no projects/environments, and minting grouped fixtures would mutate shared workspace organization (CLI has no project-create verb); membership reads were covered in sweep 489.
- Project/environment create/move/delete (REST/GraphQL-only surfaces, outside the CLI sweep scope).
