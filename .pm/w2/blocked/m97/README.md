# w2 · m97 — Blueprint preview names the reason a fetch failed

**Worker:** worker2 **Goal:** the Blueprint preview tells an author what to fix. Every fetch failure carries a coded `reason` on REST, GraphQL and MCP, the dashboard picks its title and body from that reason, and no raw `github:` error text or commit SHA reaches any surface. The existing-Blueprint sync view and the auto-sync worker classify the same failures the same way. **Status:** t001–t006 done; **t007 closeout BLOCKED** on live production verification (same credential gate as m95/m96/m100 — see the workstream README)

## Tasks (in order)

| id   | title                                                                                                                                 | est | depends_on |
| ---- | ------------------------------------------------------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | `PreviewBlueprint` classifies every fetch failure into a coded `reason` on REST, GraphQL and MCP; `error` becomes human-readable — **DONE** | 40m | —          |
| t002 | The dashboard picks title and body per reason, shows `invalid_path` beside the path field, and offers Retry only when retryable — **DONE** | 30m | t001       |
| t003 | The existing-Blueprint sync view and the auto-sync worker reuse the classifier — **DONE** | 20m | t001       |
| t004 | Render parity — **DONE** | 15m | t002, t003 |
| t005 | Simplify — **DONE** | 15m | t004       |
| t006 | Test coverage — **DONE** | 30m | t004       |
| t007 | Closeout                                                                                                                              | 10m | t006       |

## Decisions

- **Branch-vs-repo 404 is resolved from the upstream response body, not guessed from message text.** GitHub answers 404 both for a branch that does not exist and for a repository the installation cannot see, so the status alone cannot tell them apart — and t001 explicitly said to wrap the error with which lookup failed rather than guess. `resolveBlueprintCommit` now classifies on `APIError.Body` (`{"message":"Branch not found"}` vs a bare `Not Found`) and wraps with one of two exported sentinels, `github.ErrBranchNotFound` / `github.ErrRepoNotFoundOrNoAccess`. **The body never leaves the process**: it is read to pick a sentinel and is never interpolated into anything a client sees, so `w6/005`'s rule that `APIError.Error()` stays body-free is untouched.
- **A missing repository and a private repository stay one reason.** GitHub makes them indistinguishable deliberately; splitting them would turn the preview into an existence oracle for private repositories.
- **The message is built from the caller's own inputs, never from `err.Error()`.** That is what structurally guarantees no `github:`, no `unexpected status NNN` and no commit SHA can reach a client — there is no path by which upstream text becomes the message. `assertNoUpstreamLeak` pins all four bans on every classifier case.
- **`retryable` is exposed as a field, not re-derived in the client.** The dashboard must not encode which reasons are retryable; `RetryableBlueprintFetch` is the one definition, and GraphQL serves it. A future reason gains its retry behavior in one place.
- **The sync path carries the code as a leading token, not a new column.** t003 allowed a migration if the dashboard could not key on the message otherwise; it can. `blueprint_syncs.error_message` now reads `"<reason>: <sentence>"`, so a client splits on the first `": "` for a stable machine-readable code and needs no schema change. Manual sync, the reviewed-pin sync, `CreateBlueprint`'s initial fetch and the webhook-driven auto-sync worker all route through the same `blueprintFetchFailure` — the worker shares `runSync`, so it inherits it rather than duplicating the classification.
- **`invalid_path` is a field error, not a panel.** The dashboard decides which of the two path rules broke by testing the extension itself rather than matching the server's prose, and suppresses the failure panel entirely for that reason so the page reads as "fix this field".
- **An absent `reason` keeps the pre-m97 generic panel.** During a rolling deploy the dashboard can talk to an older bex-api; inventing a cause would be worse than the generic copy. Pinned by a test.

## Render comparison (t004)

Render publishes no contract for Blueprint preview failures — there is no public API operation for "preview a render.yaml before creating a Blueprint" (bex's `blueprintPreview` is a bex-native pre-create review, closest in spirit to Render's dashboard-only "Review Blueprint configurations" step), and Render's dashboard copy for a bad branch or a revoked app installation was not capturable this run (no Render account credential). So there is no divergence to record: the eight-reason vocabulary and the `reason`/`retryable` fields are a bex extension over a surface Render does not expose through its API. The `error` field keeps its existing shape for API clients, so no wire contract changed for anyone already reading it — it only stopped carrying upstream text.

## Simplify pass (t005)

- **Applied:** one classifier serves both the preview and all four sync call sites; `blueprintFetchFailure` is a two-line adapter over it rather than a second classification.
- **Applied:** `RetryableBlueprintFetch` replaces what would otherwise have been a retry-set duplicated in Go and TypeScript.
- **Declined:** collapsing `BlueprintPreviewReason` into the existing `BlueprintValidationError.Code` vocabulary. Those describe a manifest that was *fetched and parsed*; these describe never getting the file at all, and the two sets must not be confused by a client switching on one field.

## Not verified live (t007's gate)

No working production credential this run (the `.env` QA password and the CLI token are both stale), so none of the DoD's `/blueprints/new` probes were re-run against production. Each is currently backed by tests instead: the wire half by `TestPreviewBlueprintClassifiesEveryFetchFailure` (ten failure shapes, each asserting reason + sentence + no leak) and `TestPreviewBlueprintClassifiesAnInvalidPath`; the rendered half by the nine-case `blueprints-new.test.tsx` table (seven reasons, the `invalid_path` field error, and the no-reason fallback); the sync half by `TestSyncBlueprintFetchFailureRecordsErrorRun`.

## Definition of done

Run each probe on the production dashboard at `/blueprints/new`, signed in to workspace `bex`, with repository `bex-co/bex` and branch `main` unless the row says otherwise. Replaying the page's own `BlueprintPreview` GraphQL query with changed variables is acceptable for the wire checks; the title and body checks are read from the rendered page.

| input                                                        | expected title                                                          | expected body / behavior                                                                  |
| ------------------------------------------------------------ | ----------------------------------------------------------------------- | ----------------------------------------------------------------------------------------- |
| path `render.yaml` at the root, where none exists            | Blueprint file not found                                                | names the path and branch; no `github:` text, no commit SHA                               |
| path `examples/nope/render.yaml`                             | Blueprint file not found                                                | same shape with that path                                                                 |
| branch `qa-no-such-branch`                                   | Branch not found                                                        | names the branch; no `unexpected status 404`                                              |
| repo `https://github.com/bex-co/qa-no-such-repo.git`         | Repository not found, or bex's GitHub app can't access it               | one combined reason; nothing distinguishes missing from private                           |
| path `examples/hello-go`                                     | not rendered as "not found"                                             | an invalid-path message beside the path field (`.yaml`/`.yml` required)                   |
| path `../render.yaml` and `/render.yaml`                     | not rendered as "not found"                                             | an invalid-path message beside the path field (clean repository-relative path required)   |
| control: `examples/hello-go/render.yaml`                     | Blueprint file parsed successfully                                      | `found: true`; Deploy Blueprint enabled                                                   |
| control: `examples/whoami-app.yaml`                          | Blueprint file has errors                                               | Deploy Blueprint stays disabled                                                           |

Across every row above, the wire `blueprintPreview` carries `reason` set to one of the eight codes, `error` contains no `github:` prefix and no 40-hex SHA, and "Deploy Blueprint" is disabled for every failure row. A Retry control appears only for `rate_limited` and `unavailable`.

## Source + Goal linkage

- **Source:** `w1/097` (live `/qa-find-bugs` 2026-09-14 pass 26, journey 13), absorbed by `/pm-brainstorm for w1` on 2026-09-15 and materialized here per the user's `/pm for all for w2` direction. The note is retired at `.pm/w1/done/097.md`.
- **Goal linkage:** pillar 1 (Render-compatible surfaces): `docs/ADR018-render-parity.md` row 170 still marks Blueprint / `render.yaml` IaC as ◐ on every surface, and the preview's error contract is part of what keeps it there. Pillar 2 (machine-readable state): a coded reason is something an agent can branch on; an internal error string is not.
- **Expected outcome:** at the review step a Blueprint author can tell whether the path, the branch, or the repository is wrong, and API clients receive a stable `reason` instead of parsing prose. The same classification governs manual sync and auto-sync, so the leak is closed in three places at once.
- **Why now:** the classifier is one function shared by preview, sync, and the auto-sync worker; building it once now is cheaper than patching three call sites later. The current wire also carries the commit SHA `discoverBlueprintFile` pinned, which is internal state that has no business on the preview surface.
- **Render parity:** included (t004). The change adds a field to the `BlueprintPreview` shape on REST, GraphQL and MCP and changes dashboard copy, so all four surfaces must agree. Render's own preview copy for these failure cases was not captured during filing; t004 captures it and records any drift. Missing-repo and no-access stay one combined reason on purpose: GitHub returns 404 for a private repository the installation cannot see, and splitting them would turn the preview into a private-repo existence oracle.
