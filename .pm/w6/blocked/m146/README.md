# w6 · m146 — Keep service action permissions current through confirmation

Why: retrying the m143/m144 live walkthrough exposed stale permission and asynchronous confirmation paths that can send forbidden mutations after the UI context changes.

**Worker:** worker6 **Goal:** service controls dispatch only with a fresh affirmative decision for the exact current context **Status:** blocked (t001–t003, t005–t007 done; t004 live acceptance/fixture cleanup and t008 closeout await stable local infrastructure)

**Size:** ~4h40 across implementation, live acceptance and standing closing tasks; promoted from inbox 074 during the user-requested continuation on 2026-10-02.

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Bind resource permissions to fresh successful responses — **DONE** | 45m | — |
| t002 | Revalidate confirmation context after asynchronous checks — **DONE** | 40m | w6/m146/t001 |
| t003 | Apply the shared dispatch guard to header and protected dialogs — **DONE** | 50m | w6/m146/t002 |
| t004 | Complete the authenticated dev-6 permission walkthrough | 60m | w6/m146/t003 |
| t005 | Render parity — **DONE** | 20m | w6/m146/t004 |
| t006 | Simplify — **DONE** | 20m | w6/m146/t005 |
| t007 | Test coverage and required dashboard gates — **DONE** | 35m | w6/m146/t005 |
| t008 | Closeout | 10m | w6/m146/t006, w6/m146/t007 |

## Definition of done

- [x] Partial GraphQL errors, stale capability receipts and obsolete responses cannot grant service/deploy actions; resource projections follow the existing shared refresh cadence.
- [x] A confirmation that loses its workspace, resource, selected deploy or access generation before its awaited check completes sends zero mutations.
- [x] Header deploy/restart and protected retry dialogs use the shared guard, clear obsolete state, and preserve current allowed actions and chosen-commit restrictions.
- [ ] Real two-session dev-6 evidence covers the m143/m144 outstanding live states at desktop and narrow-mobile widths. Actual shared membership removal falls back to the personal workspace; any successful-empty membership injection is explicitly browser-contract evidence.
- [ ] Required dashboard gates pass, Render parity and simplify reviews are recorded, fixtures are removed, and the original live DoD records are updated honestly.

## Source + Goal linkage

- **Source:** [074](../../done/074.md), resumed by the user's `continue` after the w6 drain. The local API/CNPG gate cleared sufficiently to bring up dev-6 auth and its API with isolated real OpenFGA. The shared mock operator briefly recovered and the fixture reached Running/Ready; full live verification is now gated by renewed host/API/auth instability.
- **Goal linkage:** ADR008 API-first operation and ADR024 Render-compatible roles; the UI must consume current server authorization without inventing new permissions.
- **Expected outcome:** access loss, a failed check or a context change prevents stale UI dispatch, while authorized lifecycle/deploy actions remain usable.
- **Why now:** nine regressions failed in `resource-action-freshness.test.tsx` against the initial code: partial-error projections, workspace/generation rebinding, obsolete responses and late confirmations. Three controls pass, including ordinary transport errors, so the finding is narrower than the original suspected blanket cache-error bug.
- **Render parity:** included for tenant-facing service/deploy controls. Capability projections and the client receipt bound remain labeled bex extensions; backend roles and mutation authority are unchanged.

## Scope and evidence

This absorbs the entire live-verification follow-up; m143/m144's live boxes remain unchecked until observed. Source findings are not described as live reproductions. Keep one refresh lifecycle rather than per-control timers, preserve protected-operation and billing semantics, and do not add role-name-derived permission rules. Use only dev-6 and scoped shared mock infrastructure; do not modify another dev-N stack. No production walkthrough is authorized here.

## Blocked gate — 2026-10-02

All implementable fixes, parity review, simplify review and automated checks are complete. The shared local CAPD API and dev-6 auth databases must stay healthy long enough for the two-session browser walkthrough and cleanup. Host saturation (load above 120) caused repeated API/probe timeouts and auth database restarts despite scoped recovery. This needs local-host availability, not production credentials or a product decision. See [evidence and retained fixture inventory](evidence.md). Tasks t004 and t008 remain open; m143/m144 live boxes remain unchecked.
