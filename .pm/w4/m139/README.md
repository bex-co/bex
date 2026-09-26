# w4 · m139 — Billing's Charges card: a subscription-period total labelled "month to date" over a calendar-month tree, and a coverage watermark that never leaves the 1st

**Worker:** worker4 **Goal:** every number on the Billing page's Charges card names the window it covers, the headline and the tree beneath it cover the same window or say plainly that they do not, and `usage.coverage.through` advances with the month (or its failure to advance is traced to a named cause) **Status:** todo

## Tasks (in order)

| id   | title                                                                                       | est | depends_on       |
| ---- | ------------------------------------------------------------------------------------------- | --- | ---------------- |
| t001 | Label the rated headline by its real window, and reconcile or disclose the tree's window     | 45m | —                |
| t002 | Trace why `coverage.through` is pinned at the month start with every source degraded         | 60m | —                |
| t003 | Correct `w6/m98/t007`'s recorded mechanism for the estimate/rated gap                        | 10m | t001             |
| t004 | Render parity across REST / GraphQL / MCP / UI                                              | 20m | t001, t002       |
| t005 | Simplify                                                                                    | 15m | t004             |
| t006 | Test coverage                                                                               | 30m | t004             |
| t007 | Closeout                                                                                    | 10m | t006             |

## Definition of done

Probes were run read-only at filing from an authenticated `https://dashboard.bex.co/billing` page (pass 164, 2026-09-25, workspace `bex`, Scale plan). Nothing was created or changed.

- **The headline says which window it is.** At filing, "Total month to date **$75.30** USD" was `usage.billing.currentCost.amountUsd`, whose `periodStart`/`periodEnd` are **`2026-09-16T00:00:00Z` → `2026-10-16T00:00:00Z`**: ten days of a subscription period, not the month. Done means the label names that window (for example "Total this billing period (Sep 16 – Oct 16)"). "Month to date" is used only for a calendar-month figure.
- **The tree and the headline agree, or the card says why they don't.** At filing, the tree directly beneath summed to **$292.82** (Services $127.52 + Postgres $39.97 + Key Value $46.42 + Sandboxes $78.91; `estimatedCost.totalUsd` `292.85`) over calendar September (Sep 1–25), about 3.9× the headline. `charges-card.tsx:297-299` states that the rated total "is shown verbatim; the tree explains it", which cannot hold when the two cover different windows. Done means either the tree is computed over the rated window when a rated figure is shown, or the card states each figure's window next to it so the gap is self-explaining. t001 records which, and why.
- **Coverage advances, or its stall is named.** At filing, `usage.coverage` was `{state: "partial", through: "2026-09-01T00:00:00Z", degradedSources: [direct, http, instance, key_value, postgres, sandbox, websocket]}` on September 25. `w4/048` observed the same `through` = period start (5 sources then) early in the month, and `w4/129` recorded it again on 2026-09-21. The watermark has not moved all month. Done means that on a day after the fix deploys, `through` is within a few hours of now for this workspace, or t002 has named the stream(s) and cause pinning it, with the fix or a filed follow-up.

## Source + Goal linkage

- **Source:** live `/qa-find-bugs` pass 164, 2026-09-25 (w4-targeted, `muse.env` credentials), journey 16 (deletion propagation into usage views). Probe, re-runnable from an authenticated page:

  ```graphql
  { usage { period coverage { state through degradedSources }
            estimatedCost { totalUsd }
            billing { currentCost { amountUsd creditsAppliedUsd amountDueUsd periodStart periodEnd } } } }
  ```

  → `period "2026-09"`, `estimatedCost.totalUsd "292.85"`, `currentCost {amountUsd "75.30", creditsAppliedUsd "75.30", amountDueUsd "0.00", periodStart "2026-09-16T00:00:00Z", periodEnd "2026-10-16T00:00:00Z"}`.

- **Root cause, window mismatch:** `dashboard/src/features/usage/components/charges-card.tsx:286-300` picks `total = rated ?? sum(categories)`. The categories come from `estimatedCost` (`store.UsageMonthToDate`, calendar month start to now: `lego/backend/internal/store/usage.go`). `rated` is Stripe's subscription-period amount (`usage-page.tsx:134-136`). The label at `:396-398` is `usage.totalToDate` ("Total month to date") whenever the period on screen is the current month, whichever source `total` came from. `w6/050` already fixed the *projection* for this exact subscription-window difference, but not the label or the tree.
- **Prior disposition corrected:** `w6/m98/t007` closed the same visible gap ($78.35 vs $29.47 then) as ADR040 seal-window lag. The seal window is hours long. The measured gap here is 15 days of usage, and the API's own `periodStart` shows the mechanism: the two figures cover different windows. `w4/129` then cited that disposition to decline re-filing. t003 corrects the record so the next hunt does not inherit the wrong explanation.
- **Root cause, coverage (cause unverified):** `store.CurrentUsageCoverage` → `aggregateUsageCoverage` (`lego/backend/internal/store/usage.go`) sets `through` to the minimum `observedThrough` across active streams, so one stream with no healthy evidence since the 1st pins it there. Any existing sandbox usage marks `sandbox` degraded by design (`sandboxTotals`). Health evidence tables date from `ceddab38c` (2026-08-02), so a September gap is not a rollout artifact. Writers: `usage/service.go:833` (datastores whose display-name capture failed are recorded `unavailable`) and `:1036`. Which streams lack evidence, and why, needs production data (bex-api `usage:` log lines, `usage_source_health`/`usage_source_streams` rows) that this hunt could not read.
- **Goal linkage:** ADR040 (billing), ADR030 (pricing), `w4/048` and `w6/m98` lineage. A billing page's first duty is that its numbers mean what they say.
- **Expected outcome:** a reader can reconcile the headline with the breakdown without knowing Stripe's subscription anchor, and "Partial data" stops being a permanent badge.
- **Why now:** the gap has been visible on every hunt since at least 2026-09-01 and was twice explained away. The window evidence is now pinned to the API.
- **Render parity task included:** the UI changes. `usage` REST/GraphQL/MCP already carry `periodStart`/`periodEnd`, and t004 confirms they stay aligned (and whether MCP exposes `coverage` identically, per `w11/m8/t004`).

## Unverified

- The coverage stall's cause (t002). Nothing here asserts metering is actually wrong: the dollar amounts may be complete while only the health evidence is missing, which is exactly what t002 must separate.
- Today's deleted `qa-20260925-*` resources were not yet in the tree at filing. This is consistent with hourly metering lag and is not treated as a defect. Older deleted services appear as bare `srv-…(deleted)` rows, which is `w4/m135`'s pre-fix history.
