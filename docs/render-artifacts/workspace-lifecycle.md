# Render workspace lifecycle — live dashboard capture (settings · rename · delete)

Captured **2026-07-11** live from `dashboard.render.com` against a real authenticated session (`puncsky@gmail.com`), driving two workspaces of that account with the Playwright MCP:

- **`tea-c185th5c2rvvnhbfiltg`** — "stargately", **Professional** (paid) plan.
- **`tea-cspvkdogph6c73ft15b0`** — "Tian Pan's Workspace", **Hobby (legacy)** (free) plan.

This closes the gap `w6/m1/t001` left open (it was marked done without producing this file) and is the verbatim source `w6/m3`'s danger-zone/rename UX is reconciled against in `w6/m5/t002`. Screenshots: `.playwright-mcp/render-workspace-settings-general.png` (General settings), `.playwright-mcp/render-workspace-delete-modal.png` (delete confirmation modal). Supersedes the earlier docs-only guesses in [`../../.pm/w6/RESEARCH-workspaces.md`](../../.pm/w6/RESEARCH-workspaces.md) open question 1 ("delete/rename semantics unverified").

> **Vocabulary.** Render says **workspace** in the UI; the id prefix is `tea-` (team). Same entity bex models as `workspace`. See [`owners-api.md`](owners-api.md).

## Where the settings live

Workspace settings are at `/w/<tea-id>/settings` (page title _"Workspace settings"_). The top-left workspace dropdown (the switcher) also links **Billing** (`/w/<id>/billing`) and **Workspace settings** directly. The settings page header shows the workspace name as an `<h1>`, a plan badge linking to `/billing/update-plan`, and **Workspace ID: `tea-…`** with a **Copy** button.

Section order on the settings page (both plans, identical): **General · Team Members · Build Pipeline · Overlapping Deploy Policy · Workflows · Container Registry Credentials · Security · HIPAA Compliance · Audit Logs · Documents**. The **General** section holds the lifecycle-relevant fields.

## Rename — inline editable field, not a "rename" form

Rename is **not** a labelled "Rename" button next to a text box. In **General**, the **Name** field renders as read-only text with a small **pencil/Edit** affordance. The sequence is:

1. Click the pencil next to **Name** → the value becomes an editable textbox and two buttons appear inline: **Cancel** and **Save changes**.
2. Edit the text, click **Save changes** (or **Cancel** to abort — no-op).

There is no separate confirmation step and no character/DNS-label constraint surfaced in the UI — Render names are freeform (e.g. "Tian Pan's Workspace" contains spaces, capitals, and an apostrophe). The same inline-pencil pattern is reused for the **Email** ("Email address for workspace updates, such as system notifications") and **Avatar** fields in the same section.

## Delete — Hobby/free workspaces only, `sudo`-phrase confirmation

**Key finding: the "Delete Workspace" control is plan-gated.** It is **absent** from the General section (and from Billing) on the **Professional** workspace — a paid/primary workspace shows no delete affordance anywhere in Settings or Billing. On the **Hobby** workspace, a **Delete Workspace** button appears at the **bottom of the General section** (below the Documents section content, as the last control in `<main>`).

Clicking it opens a modal. Captured **verbatim**:

> **Delete Workspace**
>
> This will delete all of your workspace's resources and data! All services, datastores, and environment groups will be lost. Any unbilled charges will be processed at the end of the current billing period.
>
> Are you sure you want to delete this workspace?
>
> Type **`sudo delete workspace Tian Pan's Workspace`** below to confirm.

- **Confirmation field label:** _"Sudo Command"_ (a single text input, initially empty).
- **The phrase to type is `sudo delete workspace <workspace name>`** — the literal string `sudo delete workspace ` **prefixed to the workspace's own name**, not the bare name. This is the notable deviation from a plain type-the-name guard.
- **Modal buttons:** **Delete Workspace** (destructive, disabled until the phrase matches) and **Cancel**.
- **Stated resource consequences (verbatim):** _all services, datastores, and environment groups will be lost_; _any unbilled charges will be processed at the end of the current billing period_.
- **No grace period, no soft-delete, no "type DELETE"** wording — deletion is immediate on confirm. (Not confirmed live — the modal was cancelled; the copy states no grace period and offers no undo.)
- **No ownership-transfer option** anywhere in the settings/delete surface. Render has no "transfer workspace to another user" flow on this page; the only member action is the Team Members list (invite/role), gated to Pro+ for >1 member.

## Plan display (secondary observation)

The **Plan** row in General shows the plan name + tagline + an **Update Plan** link to `/billing/update-plan`:

- Professional workspace: _"Professional — For small teams and early-stage startups."_
- Hobby workspace: _"Hobby (legacy) — For hobbyists and students."_

Both carry a banner: _"We're updating our pricing on August 1, 2026."_ and an "Opt-in early / New plans, zero seat fees" nudge — the 2026-04-23 plan transition (`RESEARCH-workspaces.md` finding 1) is still mid-rollout: legacy plan names ("Professional", "Hobby (legacy)") are what the live dashboard renders today. bex clones the **new** lineup (Hobby / Pro / Scale / Enterprise), which is correct forward-looking parity — the live legacy labels are transitional.

## Switcher (secondary observation, confirms `RESEARCH-workspaces.md` findings 2–3)

The top-left dropdown is a `listbox` labelled by the current workspace. Contents, in order: **Billing**, **Workspace settings**, a **`Switch Workspace`** group listing the account's other workspaces (each with its plan as a sub-label, e.g. "Tian Pan's Workspace — hobby"), then **New Workspace** (`/new/workspace`). Switching is a direct click on the target workspace; **+ New Workspace** routes to the creation flow.

## Parity implications for bex (feed into `w6/m5/t002`)

| Render (captured live) | bex today (`w6/m3`) | Verdict |
| --- | --- | --- |
| Rename = inline pencil → textbox + **Cancel** / **Save changes** | Separate **Name** input + **Rename** button in a card | Minor drift (labelled button vs inline). Acceptable; see t002. |
| Delete confirm phrase = **`sudo delete workspace <name>`** | Type the **bare workspace name** | **Real drift** — bex's guard is weaker/different copy. Reconcile in t002. |
| Delete consequence copy names **services, datastores, environment groups** | Generic "danger zone" description | Copy drift — align wording in t002. |
| Delete only on **Hobby/free**; hidden on paid | Delete shown for any workspace | bex has no billing/subscription gating yet ("Not in w6") — deliberate deviation. |
| Names are **freeform** (spaces, caps, apostrophes) | DNS-label constrained (`^[a-z0-9]([a-z0-9-]{0,28}[a-z0-9])?$`) | Deliberate deviation, already documented (`w6/m1/t007`): bex names become App-CR names. |
| **No** ownership transfer, **no** grace period | No transfer, immediate delete | Match. |
