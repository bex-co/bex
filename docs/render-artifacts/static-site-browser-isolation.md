# Static-site browser isolation: Render vs bex

**Captured:** 2026-07-29 with installed Google Chrome, driven through its DevTools protocol by [`scripts/static-site-browser-isolation.mjs`](../../scripts/static-site-browser-isolation.mjs).

The harness maps two sibling hosts per suffix to one loopback HTTPS server. Tenant A attempts a `Secure; Domain=<suffix>` cookie, writes local storage, and registers a Service Worker. The same browser context then visits tenant B. This isolates browser suffix/origin behavior from application code and does not modify either provider's live sites.

| Observation | `tenant-a.onrender.com` → `tenant-b.onrender.com` | `tenant-a.onbex.co` → `tenant-b.onbex.co` |
| --- | --- | --- |
| Parent-domain cookie | rejected; absent on sibling | accepted; sent to sibling |
| `localStorage` | origin-local | origin-local |
| Service Worker registration | origin-local | origin-local |
| Canonical PSL exact entry | `onrender.com` present | `onbex.co` absent |

This is a genuine browser-domain divergence, not a REST/GraphQL/MCP/dashboard schema gap. bex's control plane is separately safe from tenant HTML because dashboard/API/auth use `*.bex.co`, while content uses `*.onbex.co`; the Kratos `Domain=bex.co` session cannot be sent to the latter. The owner accepted the divergence on 2026-07-30 and tenant applications must use host-only/`__Host-` cookies.

Reproduce and report the current behavior without gating PSL membership:

```sh
node scripts/static-site-browser-isolation.mjs
```

Pin the future PSL-present behavior if that decision changes:

```sh
PSL_EXPECTED=present node scripts/static-site-browser-isolation.mjs
```

The opt-in form fails until the browser-consumed PSL recognizes `onbex.co`. This artifact deliberately preserves that fact; closing w7/m54 does not assert that the two suffixes provide equivalent parent-cookie isolation.
