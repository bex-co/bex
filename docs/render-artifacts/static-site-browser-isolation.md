# Static-site browser isolation: Render vs bex

**Captured:** 2026-07-29 with installed Google Chrome, driven through its DevTools protocol by [`scripts/static-site-browser-isolation.mjs`](../../scripts/static-site-browser-isolation.mjs).

The harness maps two sibling hosts per suffix to one loopback HTTPS server. Tenant A attempts a `Secure; Domain=<suffix>` cookie, writes local storage, and registers a Service Worker. The same browser context then visits tenant B. This isolates browser suffix/origin behavior from application code and does not modify either provider's live sites.

| Observation | `tenant-a.onrender.com` → `tenant-b.onrender.com` | `tenant-a.onbex.co` → `tenant-b.onbex.co` |
| --- | --- | --- |
| Parent-domain cookie | rejected; absent on sibling | accepted; sent to sibling |
| `localStorage` | origin-local | origin-local |
| Service Worker registration | origin-local | origin-local |
| Canonical PSL exact entry | `onrender.com` present | `onbex.co` absent |

This is a genuine browser-domain divergence, not a REST/GraphQL/MCP/dashboard schema gap. bex's control plane is separately safe from tenant HTML because dashboard/API/auth use `*.bex.co`, while content uses `*.onbex.co`; the Kratos `Domain=bex.co` session cannot be sent to the latter. Tenant applications must use host-only/`__Host-` cookies during the interim.

Reproduce the dated baseline:

```sh
PSL_EXPECTED=absent node scripts/static-site-browser-isolation.mjs
```

The final production gate deliberately has no override:

```sh
node scripts/static-site-browser-isolation.mjs
```

It will fail until the browser-consumed PSL recognizes `onbex.co`. The upstream PRIVATE-section template observed on 2026-07-29 requires 2,000–3,000 distinct users plus DNS/operational evidence; production has 17 distinct tenant members. An upstream change cannot be truthfully submitted yet. Once eligible, record the accepted `publicsuffix/list` change here, wait for the target Chrome/Firefox release to consume it, then replace this table with the two-suffix passing capture.
