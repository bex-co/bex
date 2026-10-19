# CLI sweep 572 — Blueprint registry reads (list/get/syncs), read-only

**Verdict: no new bug filed** — the one anomaly seen is already tracked as w4/368 on `main` (`e870f0475`). Same isolated QA session, `bex-canary`, production pin `d98d07f62`, 2026-10-19T19:38–19:40Z. Read-only REST against the one pre-existing registered Blueprint (`blp-db136288mmqc73d4hpug`, "bex") via `/tmp/qa501/blp572.py`; the pinned CLI exposes no registry verbs (validate only). Nothing mutated; zero own footprint.

- **Get shape complete:** id/name/repo/branch/path/autoSync/manifest/status/timestamps. No-ownerId reads resolve through the session workspace; another **member** workspace's ownerId → 404 `blueprint not found` (correct scoping, non-enumerating); synthetic ownerId → 403 (FGA deny precedes lookup — same split as sweep 560); well-formed nonexistent `blp-` → the same 404; wrong-kind/malformed/uppercase ids → boundary `400 invalid path parameter "blueprintId"` (the pinned `^blp-[0-9a-z]{20}$` correction of w7/m49).
- **List paging dialects:** `limit=0/101` → named range 400; `limit=abc` → named invalid-parameter 400; garbage cursor and cursor-at-last-row → honest empty page (same lenient keyset semantics as deploys, sweep 562). Explicit `ownerId` of the other member workspace lists **its** blueprint (`blp-d9nqg95cavls73fp8m10`) — and, reconfirming sweep 563's harness note, a no-ownerId raw-REST list also resolved to that workspace despite the `X-Bex-Workspace` header; the CLI path always sends `ownerId`, so this remains a probe-scoping artifact, not product behavior.
- **Independent reproduction of w4/368:** the registry's only blueprint reads `status: in_sync`, `autoSync: true`, yet `GET …/syncs` → `[]` and the view carries no `lastSync` — the exact condition filed by the concurrent w4 loop (deploy-time auto-registration stamps `in_sync` without recording a sync). Observed, cross-checked against `origin/main`'s `.pm/w4/368.md`, deliberately not refiled.
- Syncs of a well-formed nonexistent blueprint → same `blueprint not found` 404; syncs `limit=0`/garbage-cursor behave like the list route.

Note on the user's steering mid-sweep: no "seasonal library" or TCP-SYN metric surface exists anywhere in `docs/`, `lego/`, or `dashboard/` source (targeted search; the only hits were inside `dashboard/node_modules`). Nothing measurable answers to that name, so the loop continued on its rotation.

## Unrun

- Create/sync/disconnect lifecycle against an owned repo (git-push auto-sync needs a writable GitHub repo; registry *reads* were this sweep's scope).
- Cursor walking a multi-page registry (one blueprint per workspace here).
