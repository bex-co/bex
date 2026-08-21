# w5 · m76 — Dashboard "Update Source" card: swap a service's backing repo/branch/image from Settings

**Worker:** worker5 **Goal:** the last open GitHub-integration parity item (ADR026 §8 plan item 1) closes — a service's backing source (repo + branch, or container image) is viewable and editable from its dashboard Settings page, driving the already-shipped `PATCH /v1/services/{id}` fields, with Render's "changes are not deployed automatically" semantics **Status:** todo

## Tasks (in order)

| id   | title                                                                                     | est | depends_on |
| ---- | ----------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Backend: verify + align PATCH source-swap semantics (repo↔image, no auto-deploy, ×3 surfaces) | 45m | —          |
| t002 | Dashboard: Source card on service Settings (display + Edit dialog + no-auto-deploy note)  | 90m | t001       |
| t003 | Render parity — mirror Render's Update Source dialog semantics; record any divergence      | 20m | t002       |
| t004 | Simplify — `/simplify` over the milestone's changed code                                   | 30m | t003       |
| t005 | Test coverage — swap transitions, validation, dialog wiring                                | 45m | t003       |
| t006 | Closeout — verify DoD, sync status, move to done/                                          | 15m | t005       |

## Definition of done

On a live environment: a repo-backed service's Settings page shows its Source (`owner/repo` + branch); Edit opens a dialog offering (a) a different repo from the workspace's connected accounts (account-grouped picker), (b) a different branch, (c) switching to a container image — and saving PATCHes the service **without triggering a deploy** (the next deploy uses the new source, matching Render). An image-backed service can likewise be re-pointed or switched to a repo. Validation refuses a repo whose owner matches no workspace connection with a clear message (public repos allowed as public-git, per the create flow's rules). REST/GraphQL keep identical field semantics; suites green.

## Source + Goal linkage

- **Source:** [docs/ADR026-github-integration.md](../../../docs/ADR026-github-integration.md) §8 "Render-parity status & remaining plan" item 1 (2026-08-21), grounded in the live Render-dashboard walk: Render's Settings → Source **Edit** (May-2026 "Update Source" changelog) swaps repo/branch or repo↔image; bex already has the API half (`PATCH /v1/services/{id}` accepts `repo`/`branch`/`image` since w1/073 matched Render's PATCH field set) but no dashboard affordance.
- **Goal linkage:** Render parity (ADR018) on the service-configuration surface; completes the GitHub-integration parity plan — after this, ADR026 §8's "remaining" list is empty except the unscheduled grant-staleness polish.
- **Expected outcome:** a mis-pointed or migrated service is fixed from the UI in seconds (today it needs a raw API call); the ADR026 §8 plan item closes.
- **Why now:** it is the single remaining functional gap from the 2026-08-21 parity audit, small and well-bounded, and the m74 account-grouped repo picker it reuses just shipped.
- **Render parity task included:** yes — service-facing UI + the PATCH surface semantics must stay Render-consistent across REST/GraphQL.

## Constraints

- **No new backend verbs** — the PATCH fields exist; t001 only verifies/repairs semantics (e.g. repo⇄image transitions clearing the other source, `autoDeploy` untouched, no implicit deploy).
- Reuse `ServiceSourcePicker` (account-grouped, workspace-scoped per ADR078 §6) rather than a second repo picker.
- Type is immutable (Render: "You cannot change an existing service's type") — the dialog never offers a type change.
