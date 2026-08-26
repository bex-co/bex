# w6 · m102 — Hydration mismatch (#418) in `formatRelativeAge`/`formatRelativeUntil` across 15+ components

**Worker:** worker6 **Goal:** no route that renders a relative-age/until timestamp during its blocking SSR pass can produce a React hydration-mismatch console error, regardless of the timing gap between server render and client hydration **Status:** in progress — code complete and shipped-ready; t003 (live probe) and t007 (closeout) blocked until this deploys

## Tasks (in order)

| id   | title                                                                                          | est | depends_on |
| ---- | ------------------------------------------------------------------------------------------------ | --- | ---------- |
| t001 | Extract a shared hydration-safe wrapper for `formatRelativeAge`/`formatRelativeUntil` output      | 30m | — | — **DONE**
| t002 | Apply the wrapper to every currently-unguarded call site (22 across 15 files)                     | 1h  | t001 | — **DONE**
| t003 | Live verification: repeat the boundary-crossing repro on 3+ routes, confirm zero #418              | 30m | t002 | — **BLOCKED** (needs deploy)
| t004 | Render parity — confirm no REST/GraphQL/MCP wire-shape or UI-copy change, rendering-only fix        | 15m | t003 | — **DONE**
| t005 | Simplify                                                                                          | 20m | t004 | — **DONE**
| t006 | Test coverage                                                                                     | 30m | t004 | — **DONE**
| t007 | Closeout                                                                                          | 10m | t005, t006 | — **BLOCKED** (gated on t003)

## Definition of done

- Fresh `page.goto()` to `/webhook/<id>` (any webhook with a delivery sent within the last ~2 minutes), with `browser_console_messages` checked after the page settles, shows **zero** React error #418 — repeated at least 3x across a real run to cross a bucket boundary (60s/60min/24h/30d) during the load, matching this hunt's actual repro (a delivery `sentAt` ~55s before load, crossing the "now"→"1m" boundary).
- The same check, repeated against at least 2 more of the newly-guarded routes (e.g. a cron job's `/services/<id>` header with a recent run, and `/databases/<id>`), shows zero #418.
- `grep -rn "formatRelativeAge(\|formatRelativeUntil(" dashboard/src --include="*.tsx" --include="*.ts" | grep -v __tests__` — every result sits inside a `suppressHydrationWarning`-guarded element (directly, or via the new shared wrapper) — re-running this hunt's proximity-check script (below) against the fixed tree returns zero `guarded=NO` lines.

  ```bash
  for f in $(grep -rl "formatRelativeAge(\|formatRelativeUntil(" dashboard/src --include="*.tsx" --include="*.ts" | grep -v "__tests__\|lib/format.ts"); do
    awk '
      /suppressHydrationWarning/ { guard[NR]=1 }
      { lines[NR]=$0 }
      /formatRelativeAge\(|formatRelativeUntil\(/ { calls[NR]=1 }
      END { for (n in calls) { found=0; for (g in guard) if (g>=n-6 && g<=n+2) found=1
        printf "%s line %d: guarded=%s\n", FILENAME, n, (found?"YES":"NO") } }
    ' "$f"
  done
  ```

- `yarn typecheck && yarn lint && yarn test` (dashboard) green.

## Source + Goal linkage

- **Source:** live QA hunt of `dashboard.bex.co`, 2026-08-25/26 (10th hourly run). Reproduced live: navigating to `/webhook/whk-da78c6hgoibs73ah08u0` (a QA-created webhook, deleted after the hunt) immediately after a delivery landed ~55 seconds earlier logged `Minified React error #418` in the browser console (saved: `.playwright-mcp/qa-webhook-react418-console.log`). Traced to `dashboard/src/features/webhooks/components/webhook-deliveries-card.tsx:317-320` (`<time dateTime={delivery.sentAt}><span>{formatRelativeAge(delivery.sentAt)}</span>...`, no `suppressHydrationWarning`) calling `dashboard/src/features/services/lib/format.ts:25-46` (`formatRelativeAge`, defaults `now: number = Date.now()` — evaluated independently at SSR-render time and at client-hydration time; any elapsed-time bucket boundary (60s/60min/24h/30d/12mo) crossed in the gap between the two produces different server/client text for the same `<span>`, which is exactly what React error #418 flags).
- **Not a duplicate of, or fix-in-progress for, `w6/030`** (open inbox note; its code fix has already landed on `main`, confirmed live this run: `dashboard/src/features/env-groups/components/env-group-metadata.tsx:50-59` carries a `suppressHydrationWarning` guard with an explicit `w6/030` comment). The two bugs share the generic React #418 code but have **distinct mechanisms**: w6/030 is `formatDateTime`'s timezone-of-instant divergence (the SSR container renders in UTC, the browser renders in the visitor's local timezone — same instant, different string). This milestone's bug is `formatRelativeAge`/`formatRelativeUntil`'s elapsed-time-since-render divergence (`Date.now()` ticks forward between SSR and hydration — same code, different *instant it's compared against*). This is exactly the same reasoning `w6/030` itself used to distinguish its own env-groups finding from `w1/done/m81/done/t002.md`'s unrelated `global-search.tsx` fix — a third, independent root cause under the same generic error code.
- **Root cause:** `dashboard/src/features/services/lib/format.ts:25-46` (`formatRelativeAge`) and `:51-68` (`formatRelativeUntil`) both accept an injectable `now` (used by their own unit tests for determinism) but every production call site omits it, silently defaulting to a fresh `Date.now()` per call, per render pass.
- **Blast radius — exhaustively grepped** (`grep -rn "formatRelativeAge(\|formatRelativeUntil(" dashboard/src --include="*.tsx" --include="*.ts" | grep -v "__tests__\|lib/format.ts"`, 27 call sites, 20 files): 2 call sites (in `features/projects/components/resource-table.tsx:224,235` and `features/services/components/service-detail-header.tsx:454`) are already incidentally guarded — they got a `suppressHydrationWarning` wrapper as a side effect of the same files being touched for their own unrelated `formatDateTime`/fact-list guards, not because this mechanism was deliberately fixed. The remaining **22 call sites across 15 files** are unguarded:
  - `features/databases/components/recovery-panel.tsx:228,350`
  - `features/ssh-keys/components/ssh-keys-panel.tsx:186`
  - `features/connected-agents/components/connected-agent-row.tsx:63`
  - `features/api-keys/components/api-key-row.tsx:24,31`
  - `features/sessions/components/session-row.tsx:33`
  - `features/agent-sessions/components/session-list.tsx:242,317`
  - `features/events/components/event-timeline.tsx:97`
  - `features/webhooks/components/webhook-row.tsx:80`
  - `features/webhooks/components/webhook-deliveries-card.tsx:318` (live-reproduced this run)
  - `features/registry-credentials/components/registry-credential-row.tsx:44`
  - `features/services/components/cron-runs-section.tsx:185`
  - `features/services/components/service-detail-header.tsx:213,221` (a **different** block in the same file than the guarded line 454 — the cron last-run/next-run header fields)
  - `routes/keyvalue.$keyValueId.tsx:285`
  - `routes/blueprints.tsx:157`
  - `routes/blueprints.$blueprintId.tsx:363,373,450,455`
  - `routes/services.$serviceId.events.tsx:391`
  - `routes/databases.$databaseId.tsx:380`
- **Goal linkage:** dashboard SSR correctness — no single ADR governs this; it's TanStack Start SSR hygiene, the same class of concern as `dashboard/CLAUDE.md`'s documented "Navigation pending states" and "SSR gotcha" sections.
- **Expected outcome:** the `formatRelativeAge`/`formatRelativeUntil` half of the recurring-#418 pattern is closed in one pass via a shared, guard-baked-in component, so a future call site can't reintroduce it by omission.
- **Why now:** this is the **third** independently-discovered #418 root cause on this dashboard (after `w1/done/m81/done/t002.md`'s `navigator.platform` read and `w6/030`'s `formatDateTime` timezone divergence) sharing the same generic error code — each has so far been fixed one file at a time instead of as a class. Both already-completed guarded sites landed as a byproduct of unrelated work, showing the pattern isn't yet reliably applied by convention alone; wrapping it in a shared component (t001) closes the whole class at once and prevents a fourth recurrence.
- **Render parity:** included (t004) as a quick confirmation only — this is a pure rendering-timing fix with no REST/GraphQL/MCP payload or dashboard-copy change; t004 verifies that stays true.
- **Adjacent classes:** n/a — rendering-only, no auth/error-taxonomy surface.
- **Unverified:** only 1 of the 22 unguarded sites (`webhook-deliveries-card.tsx:318`) was independently reproduced live this run; the other 21 are inferred from the identical code shape (an unguarded `Date.now()`-based formatter rendered during a route's initial load) but not individually navigated to and observed this run. It's also unverified whether every one of the 22 sites is actually reached during a route's *blocking* SSR pass (only that path can manifest #418) versus only after client-side interaction (e.g. a dialog opened post-mount, which can't hydration-mismatch since there's no server-rendered counterpart to diverge from) — t002/t003 should confirm SSR-reachability per site before/while applying the guard, and drop any site from the "fixed" count that turns out to be client-only.

## Progress (2026-08-26)

**The class is closed in code.** `formatRelativeAge(`/`formatRelativeUntil(` now appear in exactly one place outside their own module — `dashboard/src/common/components/relative-time.tsx`, which bakes in `suppressHydrationWarning` and the machine-readable `dateTime`. All **27** call sites were migrated, not just the 22 unguarded ones: the 2 incidentally-guarded sites in `resource-table.tsx` and the 3 fact-object sites were folded in too, so no site depends on a guard someone else happened to add. The proximity-check script above now returns zero lines.

- **DoD bullet 3** (zero `guarded=NO`) — **met.**
- **DoD bullet 4** (`yarn typecheck && yarn lint && yarn test`) — **met**, 372 files / 2692 tests green.
- **DoD bullets 1–2** (live `/webhook/<id>` + 2 more routes) — **not met, and not meetable pre-deploy**: production serves the pre-fix bundle. See `t003.md`. In their place, `dashboard/src/test/hydration.ts` forces the boundary crossing deterministically (SSR at one clock, hydrate at another) on three surfaces, with a negative control proving the probe still detects an unguarded formatter — permanent CI coverage the one-shot live probe never gave.

**One scope correction worth carrying forward.** This README's blast-radius analysis was written before `w6/m107` was filed. The `formatDateTime`/`formatDateLong` renders that sit *on the same elements* as several of these ages (the webhook detail header's created date, the exact timestamp beside each delivery age) are m107's class, not this one. Guarding them was tried and reverted: m107's own finding is that `suppressHydrationWarning` is the wrong fix there, because it freezes the SSR container's UTC text instead of letting React's mismatch re-render the viewer's local time — it would have traded a console error for a wrong value. Consequence: **`/webhook/<id>` may still log #418 after this deploys, and that residue is m107's.** The two are distinguishable — m107's vanishes when the viewer's timezone is UTC; this one's does not.

**One unavoidable spillover:** `services.$serviceId.events.tsx:389` carried `title={exactTimestamp}` on the same element as its relative age, so the wrapper's guard now covers that attribute too. m107 should re-check that tooltip.

**Unverified item resolved, partially.** `recovery-panel.tsx`'s 2 sites are behind `React.lazy` (`databases.$databaseId.tsx:49`) and never render in the blocking SSR pass, so they could not have been producing #418 — migrated for uniformity, but they should not be counted as fixed live defects. `webhook-deliveries-card.tsx` is confirmed SSR-reached (its test asserts the server HTML contains `>now<`). The remaining sites were not individually probed; the wrapper makes the question moot for them.
