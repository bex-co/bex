# w4 · m161 — Bind environment-group revisions to editing drafts

**Worker:** worker4 **Goal:** an older dashboard draft cannot silently overwrite a newer saved group value after polling **Status:** blocked — t001–t005 done 2026-10-03; t006 live two-tab acceptance and closeout remain

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 — **DONE** | Audit draft, revision, restoration and shared-editor boundaries | 25m | — |
| t002 — **DONE** | Keep the group draft's base revision until save or discard | 55m | w4/m161/t001 |
| t003 — **DONE** | Verify API and Render parity without weakening conflicts | 20m | w4/m161/t002 |
| t004 — **DONE** | Simplify the draft revision lifecycle | 15m | w4/m161/t003 |
| t005 — **DONE** | Test polling, conflicts, recovery and unaffected callers | 45m | w4/m161/t003, w4/m161/t004 |
| t006 | Closeout after live two-tab acceptance | 15m | w4/m161/t005 |

Total: **6 tasks, about 2h 55m**.

## Definition of done

- Repeat the two-tab journey in [finding.md](finding.md): A edits QA_CONFLICT, B saves a different value, A receives a real EnvGroup poll with B's revision. A's draft stays visible, but saving still carries A's original base revision. The server rejects it with ENV_GROUP_REVISION_CONFLICT; B's persisted value and revision remain unchanged.
- Repeat from fresh pages, as in the second observed reproduction. The refused draft remains available for the user to copy or discard; no automatic rebase, replay or overwrite occurs. Starting a fresh edit after explicit discard reads the current revision and saves normally.
- Preserve the exercised direct-client controls: stale GraphQL, REST PATCH /v1/env-groups/{id}/contents and MCP patch_env_group_environment requests are refused without changing value or revision.
- Keep the sweep-48 controls: canceled edits do not persist; Save only preserves untouched masked values and exact multiline file contents. Do not reveal existing secrets merely to initialize or refresh a draft.
- Delete every owned fixture and revoke its session. Record hosted results and relevant automated gates before moving this milestone to done.

Reauthentication, file-only conflicts, linked-service rollout/retry, missing-revision compatibility and service-editor behavior need implementation regression coverage; they were not live conflict probes in this filing.

## Source + Goal linkage

- **Source:** user-directed continuous $qa-find-bugs using muse.env, sweep 49, 2026-10-02 local / 2026-10-03 UTC. Two independent fresh-page UI reproductions and direct three-surface controls are preserved in [finding.md](finding.md).
- **Goal linkage:** ADR008 reliable managed hosting; ADR006 revision-aware sparse group contents; ADR013 shared configuration custody; ADR018 environment-group parity.
- **Expected outcome:** the revision describes the state from which a draft began, so concurrent changes cause an honest refusal rather than a successful lost update.
- **Why now:** the API already protects stale writes, but the dashboard substitutes the latest polled token while preserving old edits. Shared configuration can silently lose another editor's saved value.
- **Dedupe:** uncovered dashboard gap in w2/m73's staged revision-aware workflow, not the backend metadata CAS work in w4/m97 or service post-write conflict issue w4/m130. The original revision-ref assignment is still present from 4f3619d1f; no later fix was found. Preserve w1/m153's mounted drafts across polls.
- **Render parity included:** dashboard behavior changes while existing REST/GraphQL/MCP error contracts remain correct. Render documents shared group editing; its concurrent-tab conflict behavior was not tested or asserted.
- **Scope:** filing only, no product implementation. Fixture evg-db09u9atm2ss7389qn2g was unlinked, deleted through the UI, returned API404, left no matching projected Secrets, and its session was revoked.
