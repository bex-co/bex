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

## Rule 4 — a deleted service's subdomain is never reused

**Captured:** 2026-11-15 · **Method:** Render Support staff answers on Render's community forum (sunset 2026-03-24, [render.com/docs/community](https://render.com/docs/community); the threads were read from their Internet Archive copies) plus first-hand user reports. Render's docs only say "Every Render web service gets a unique `onrender.com` subdomain" ([render.com/docs/web-services](https://render.com/docs/web-services)) and never say whether a deleted service's subdomain is released. **Grade:** answers from three Render staff over 2024–2025 (`jeremy-render` and `John_B` sign their posts "Render Support"; `mmaddex` answers for Render, "we can't restore the old url"), consistent with each other, plus first-hand user reports and no contrary report found. Strong, but not an official doc statement.

- [After how much time onrender release its deleted domains to be used again](https://community.render.com/t/after-how-much-time-onrender-release-its-deleted-domains-to-be-used-again/24465) (archived [2024-12-07](https://web.archive.org/web/20241207210618/https://community.render.com/t/after-how-much-time-onrender-release-its-deleted-domains-to-be-used-again/24465)). The user (2024-08-31): "i deleted a webservice which have some specific https://www.xxxx.onrender.com after how much time render takes to release its deleted domains to be used again". `jeremy-render`, signed "Render Support" (2024-09-02): "The onrender.com address is not re-usable, the only control you have is over the custom domains attached to your services."
- [Reusing Old URL](https://community.render.com/t/reusing-old-url/26327) (archived [2025-03-15](https://web.archive.org/web/20250315040906/https://community.render.com/t/reusing-old-url/26327)). First-hand (2024-11-01): "I uploaded a new web service and deleted the one with the old URL … When I attempt to make a new dashboard under this new URL, Render states that the URL is in use, though I've deleted the URL and have nothing running with it." `mmaddex` for Render (2024-11-01): "I'm afraid we can't restore the old url. The best approach would be to use a Custom Domain."
- [Render Custom Domain](https://community.render.com/t/render-custom-domain/37522) (archived [2025-09-08](https://web.archive.org/web/20250908235841/https://community.render.com/t/render-custom-domain/37522)). A user who re-created a service as Docker (2025-06-08): "Now if I want to switch the same domain name, it is not allowing me." `John_B` (2025-06-10): "the part before the onrender.com address has to be unique and can't be reused - you should consider using a custom domain here so you are in control of the domain."
- Consistent with these: [Can I change my existing subdomain](https://community.render.com/t/can-i-change-my-existing-subdomain-onrender-com-to-a-new-one/9999) (`al_ps`, 2023-03-07: "The service subdomain has to be unique across the platform and cannot currently be changed once the service is created … if it's in use by another account a random suffix will be added") and [How do I change URL of a web service](https://community.render.com/t/how-do-i-change-url-of-a-web-service/11980) (`John_B`, 2023-05-12: "you can change the name but that does not change the slug").

So a slug is taken for good the moment a service gets it. Re-creating a deleted service's name gives a suffixed subdomain, in the same workspace or any other. Render's own [Blueprint example fix](https://github.com/render-examples/pydantic-agents-workflows/pull/22) (2026-11-04) says the same from the other side: "The URL also differs per deploy: if the name is taken, Render adds a suffix".

**What would settle it beyond doubt:** on a Render account, create a free static site with an unused name, note its `onrender.com` host, delete it, create another static site with the same name, and read the new host (and `RENDER_SERVICE_NAME`, Rule 5, from a web service done the same way).

## Rule 5 — `RENDER_SERVICE_NAME` is the slug, suffix included

**Captured:** 2026-11-15. **Grade:** an inference from two official docs, confirmed by a first-hand environment dump and a second user's report.

- The official variable list defines `RENDER_SERVICE_NAME` as "A unique, human-readable identifier for a service" and `RENDER_DISCOVERY_SERVICE` as having "the format `$RENDER_SERVICE_NAME-discovery`" ([render.com/docs/environment-variables](https://render.com/docs/environment-variables)). The private-network doc gives the same hostname "the format `[INTERNAL_HOSTNAME]-discovery` (e.g., `myapp-ne5j-discovery`)", and its internal hostnames carry the suffix (`elasticsearch-2j3e:9200`, [render.com/docs/private-network](https://render.com/docs/private-network)). So `RENDER_SERVICE_NAME` is the internal hostname, which is the slug.
- First-hand ([swyx.io/django-on-render](https://swyx.io/django-on-render), 2020-02-07), a printed environment: `RENDER_SERVICE_NAME: 'django-test-9g3f'`, `RENDER_EXTERNAL_HOSTNAME: 'django-test-9g3f.onrender.com'`, `RENDER_DISCOVERY_SERVICE: 'django-test-9g3f-discovery'`.
- A user's comment (zlotnika, 2022-10-05) on the [preview-domain gist](https://gist.github.com/johnbeynon/8accbb18c4791c0ef4449161af34dc94) by `johnbeynon`, which builds a hostname from `ENV['RENDER_SERVICE_NAME']`: the service id can be found by calling the list-services API and "filtering on `slug` == `RENDER_SERVICE_NAME`".

A renamed service keeps its slug (Rule 4's second link), so its `RENDER_SERVICE_NAME` keeps the slug too, not the new name.

## Summary: name vs. slug

| Axis | Scope | Enforcement | Example |
| --- | --- | --- | --- |
| `name` | Per-workspace | Reject on duplicate; dashboard suggests `name-N` | `beancount-cms` used once per workspace |
| `slug` (public host, `RENDER_SERVICE_NAME`) | Global (all workspaces), for good | Silent random suffix when any service holds or ever held it, transparent to the creator | `beancount-cms-bkxk.onrender.com` |

## bex parity decisions (w4/m19)

| Render behavior | bex implementation |
| --- | --- |
| Workspace-unique name, reject on duplicate | ✅ `Service.Create` returns 409 "name is already in use" for a same-tenant duplicate (t003) |
| Cross-workspace same name allowed | ✅ collision-free CR naming so two tenants both create `beancount-cms` (t003) |
| Globally-unique slug, random suffix on collision | ✅ `apps.slug` column, minted `-xxxx` on global collision, drives the platform host (t002, t004) |
| A deleted service's slug is never reused (Rule 4) | ✅ since 2026-11-15: the `service_slugs` ledger (migration `0278_service_slugs`) keeps every slug ever minted, so a re-created or another workspace's same-named service gets `-xxxx` |
| `RENDER_SERVICE_NAME` is the slug (Rule 5) | ✅ `lego/types/renderenv` sets it from the slug (w5/m148) |
| Dashboard inline "Name is already in use" + suggestion | ✅ debounced availability query, inline error + suggestion chip (t005, t006) |

See [`new-service-wizard.md`](new-service-wizard.md) for the surrounding create-form design this behavior slots into.
