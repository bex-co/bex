# w4 · m183 — Make SSH remedies truthful and usable

**Worker:** worker4 **Goal:** a tenant can understand the remaining shell-access blockers and follow a Connect remedy without leaving the destination behind an open modal menu. **Status:** blocked

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Explain both suspension and the Free-plan shell restriction — **DONE** | 30m | — |
| t002 | Dismiss Connect when its remedy link navigates — **DONE** | 40m | — |
| t003 | Verify shared callers and the original SSH-remedy guarantees — **DONE** | 30m | w4/m183/t001, w4/m183/t002 |
| t004 | Render parity — **DONE** | 20m | w4/m183/t003 |
| t005 | Simplify — **DONE** | 15m | w4/m183/t004 |
| t006 | Test coverage — **DONE** | 35m | w4/m183/t004 |
| t007 | Closeout | 10m | w4/m183/t005, w4/m183/t006 |

## Definition of done

Repeat the owned Free BusyBox fixture journey in [finding.md](finding.md), including its healthy served revision, canceled exit-42 command, and suspend/resume sequence. Clean it up afterwards. Check at desktop 1440×1000 and narrow mobile 390×844, waiting for the destination data before inspecting its controls:

- With `plan: free`, `suspended: suspended`, and empty `sshAddress`, Shell and Connect visibly explain **both** suspension and the paid-plan requirement. A Resume link may restore service traffic; the text must not say or imply that resuming this Free service enables SSH or the browser shell. After Resume settles Running, the service still has `plan: free` and empty `sshAddress`, the prior marker serves HTTP 200, and Shell offers Change instance type rather than a terminal or SSH command.
- On the running Free service's Shell page, Connect → Change instance type navigates to `/services/<id>/plan` and dismisses Connect. After the picker is ready, its named radiogroup is available in the accessibility tree, no Connect menu remains open, and the modal's body pointer lock is released. This matches the directly clicked Shell-page remedy control observed during the hunt.
- On the suspended Free service's Shell page, Connect → Resume service navigates to `/services/<id>/settings#suspend` and dismisses Connect. After Settings is ready, its enabled Resume button is accessible without a second outside click or Escape. Activating it restores the healthy served marker and retains the Free shell restriction. The direct Shell-page Resume link remains usable.

Paid service execution, no-key paid CTA navigation, Chinese UI, static/cron exclusions, datastore siblings, Render aliases, and pristine deployment-history fixtures were not exercised live in this cycle. They are explicit verification work in t003/t004, not claims of observed production success.

## Source + Goal linkage

- **Source:** user-requested infinite `qa-find-bugs` loop, filings in w4, 2026-10-08 UTC, cycle 11. Production workspace `bex` / `tea-d98210cbbpdc73dcrkvg`. Login helper consumed the user-specified `muse.env` privately. Two minor findings; complete matching GraphQL requests/responses and selected DOM/accessibility observations are preserved in [finding.md](finding.md).
- **Goal linkage:** ADR008 reliable hosting and actionable supervision; ADR035 running-instance SSH and browser Shell; ADR018 parity; ADR007 suspension restores serving rather than changing the plan.
- **Expected outcome:** the tenant sees all known blockers in the combined Free/suspended state, and following a Connect remedy makes the destination usable immediately after normal loading.
- **Why now:** the existing remedies are the tenant's path out of a shell refusal. One promises an outcome the correct backend gate forbids; the other retains a modal overlay on the very page it recommends.
- **Precedent:** w4/done/143 and commit `2fbcbf2ac` restored visible reasons and remedy links. These are uncovered intersections/navigation behavior in that implementation, not a recurrence of the original hover-only explanation bug. t003 audits every guarantee in that note without claiming unprobed paid states passed.
- **Render parity included:** dashboard-only fixes, preserving current REST/GraphQL/MCP plan and authorization gates. [Render SSH documentation](https://render.com/docs/ssh) excludes Free web services from native SSH and dashboard shell. Its documentation does not establish the exact Connect remedy text or dismissal behavior, and no authenticated Render UI comparison was performed.
- **Sizing:** 100m across two fixes and shared-caller verification, plus 80m closing work: seven tasks, approximately 3h. This is a coherent pair of defects in the same remedy journey, with distinct causes and evidence.

## Progress (2026-10-08)

t001–t006 done.

- **t001:** `sshRemedy` checks suspended **and** Free first. It returns the new `services.sshUnavailableSuspendedFree` (en and zh), which names both blockers and says resuming restores traffic only, with the Resume remedy. A suspended paid service keeps "Resume it to connect over SSH", and a running Free service keeps Change instance type. The Shell page and the header Connect menu both render `sshRemedy`, so they move together.
- **t002:** the header's Connect `DropdownMenu` is now controlled and closes on any click on an `a[href]` inside its content. That covers the remedy link, the no-key add-key CTA and the browser-terminal link, which all navigate while `ServiceDetailLayout` keeps the header mounted. Ordinary open menus keep their modal behavior, and nothing is remounted.
- **t003–t004:** parity holds. Render excludes Free services from SSH and the dashboard shell, and REST/GraphQL/MCP gates are unchanged.

Tests in `service-detail-header.test.tsx`:

- A combined-blocker case: both reasons shown, no "Resume it to connect" text, Resume links to `settings#suspend`.
- A real-router layout case that keeps the header mounted and clicks each remedy. The destination renders, the menu closes and the body pointer lock is released. Both cases **fail without the close handler**.

Dashboard `yarn lint` and the full `yarn test` (4401) pass.

Not verified until the live replay: desktop and 390 px replays, the paid no-key CTA navigation, and zh rendering.
