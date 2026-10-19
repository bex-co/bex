# CLI sweep 574 — Blueprint registry writes: every fetch failure is a bare 500

**Verdict: one bug filed — [w8/134](../../134.md).** Same isolated QA session, `bex-canary`, HEAD-built CLI at `77ec448ae`, production pin `d98d07f62`, 2026-10-19T19:50–19:57Z. Same-credential REST (`/tmp/qa501/blpw574.py`); the pinned CLI has no blueprint create/sync verbs. Nothing was created: the registry still holds only the pre-existing `bex` blueprint, and no fixture was needed.

The finding (full research in the note): `POST /v1/blueprints` answers **every** manifest-fetch failure with `500 {"error":"internal error"}` — nonexistent repo, nonexistent branch, missing file, wrong extension, path escape, ssh-form repo, even a non-URL repo string. `POST /v1/blueprints/preview` on the identical inputs returns the coded truth (`file_not_found`, `branch_not_found`, `repo_not_found_or_no_access`, `invalid_path`, each with a user-safe sentence). Root cause: `CreateBlueprint` (`blueprint.go:1085`) returns `blueprintFetchFailure` — the no-sentinel `"<reason>: <sentence>"` adapter built for `blueprint_syncs.error_message` (w2/m97) — as its HTTP error, so `core.WriteErr` classifies it unclassified-500 and suppresses the text. m97's own README records the create path sharing the classifier; the HTTP status mapping for create was never added.

Correct behavior observed around it (not filed):

- Missing repo/branch → named 400 before any fetch; synthetic `ownerId` → 403 at authorize.
- PATCH/sync on a well-formed nonexistent `blp-` → 404 `blueprint not found`; DELETE same → plain 404; malformed id on PATCH → boundary `400 invalid path parameter "blueprintId"` (sync's malformed id reaches its handler and 404s — the `{id}/sync` route has no path gate entry; harmless shape difference, recorded here rather than filed).
- An unknown body field on create was tolerated (the request proceeded to the fetch and 500'd there) — this route's body is decoded leniently, unlike gated Render ops.
- The existing blueprint was never synced, patched, or disconnected.

Method note: the first preview round mistakenly sent `filePath` (the handler reads `path`), so three cells measured default discovery; they were re-run with the correct field before filing — only the corrected results are cited.

## Unrun

- A successful create (needs a repo+branch+path whose manifest fetches and is safe to apply — would also deploy a stack; the refusal surface was this sweep's scope, and w8/134 blocks diagnosing it from the API today anyway).
- The already-connected 409 happy refusal (unreachable here: the one connected repo's manifest no longer fetches, and the correct takeover confirm must never be sent against a baseline).
