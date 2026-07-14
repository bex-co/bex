# Capture — Render's duplicate-service-name behavior (w4/m19 t001)

**Captured:** 2026-07-13 · **Method:** docs-fallback (Render's public docs) + user-observed dashboard behavior. The design source for bex's w4/m19 (workspace-unique names + globally-unique subdomains).

## Rule 1 — service names are unique per workspace, not per project/environment

Render's docs state it plainly:

> "All of a workspace's services must have unique names—even services that belong to different project environments." — [render.com/docs/projects](https://render.com/docs/projects)

So the uniqueness scope is the **workspace** (bex: tenant), not any narrower grouping. Two services in the same workspace can never share a name, regardless of which project or environment each belongs to. Two different workspaces, however, are free to both use the same name — Render has no cross-workspace uniqueness constraint at all.

## Rule 2 — the public subdomain is a `slug`, distinct from `name`, made globally unique with a random suffix

Render's public URL is not `<name>.onrender.com` — it's `<slug>.onrender.com`, where `slug` defaults to the name but gets a random short suffix appended the moment a bare-name collision would occur across the whole platform (Render's `onrender.com` namespace is global, spanning every workspace).

Evidence:

- Render's own quick-start doc shows the pattern directly: a service named `helloworld` ends up live at `helloworld-p9vq.onrender.com` ([render.com/docs/your-first-deploy](https://render.com/docs/your-first-deploy)).
- User-observed (2026-07-12): a service named `beancount-cms` was live at `beancount-cms-bkxk.onrender.com` — same shape, four-character random suffix, hyphen-separated.

The suffix is short (4 chars in both observed examples), looks like a random base36/base62 token, and is only appended when needed — Render does not suffix every service, only the ones that would otherwise collide globally.

## Rule 3 — the dashboard rejects duplicates inline and offers a concrete suggestion

User-observed dashboard behavior (2026-07-12): typing a name already in use in the current workspace's create-service form shows an inline **"Name is already in use"** message under the name field — not a toast, not a failed submit. Alongside the error, Render suggests a free alternative by appending a numeric counter to the taken name:

- `beancount-cms-v2` is taken → the form suggests `beancount-cms-v2-1`.
- Accepting the suggestion (one click) fills the input with a name that is guaranteed free, and the form unblocks submission.

This means the availability check runs client-side as the user types (debounced), ahead of any create attempt — a duplicate name never reaches the create endpoint from the normal UI flow.

## Summary: name vs. slug

| Axis | Scope | Enforcement | Example |
| --- | --- | --- | --- |
| `name` | Per-workspace | Reject on duplicate; dashboard suggests `name-N` | `beancount-cms` used once per workspace |
| `slug` (public host) | Global (all workspaces) | Silent random-suffix on collision, transparent to the creator | `beancount-cms-bkxk.onrender.com` |

## bex parity decisions (w4/m19)

| Render behavior | bex implementation |
| --- | --- |
| Workspace-unique name, reject on duplicate | ✅ `Service.Create` returns 409 "name is already in use" for a same-tenant duplicate (t003) |
| Cross-workspace same name allowed | ✅ collision-free CR naming so two tenants both create `beancount-cms` (t003) |
| Globally-unique slug, random suffix on collision | ✅ `apps.slug` column, minted `-xxxx` on global collision, drives the platform host (t002, t004) |
| Dashboard inline "Name is already in use" + suggestion | ✅ debounced availability query, inline error + suggestion chip (t005, t006) |

See [`new-service-wizard.md`](new-service-wizard.md) for the surrounding create-form design this behavior slots into.
