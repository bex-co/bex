# Static-site browser isolation: Render vs bex

**Captured:** 2026-07-29 with installed Google Chrome, driven through its DevTools protocol by [`scripts/static-site-browser-isolation.mjs`](../../scripts/static-site-browser-isolation.mjs).

The harness maps two sibling hosts per suffix to one loopback HTTPS server. Tenant A attempts a `Secure; Domain=<suffix>` cookie, writes local storage, and registers a Service Worker. The same browser context then visits tenant B. This isolates browser suffix/origin behavior from application code and does not modify either provider's live sites.

| Observation | `tenant-a.onrender.com` → `tenant-b.onrender.com` | `tenant-a.onbex.co` → `tenant-b.onbex.co` |
| --- | --- | --- |
| Parent-domain cookie | rejected; absent on sibling | accepted; sent to sibling |
| `localStorage` | origin-local | origin-local |
| Service Worker registration | origin-local | origin-local |
| Canonical PSL exact entry | `onrender.com` present | `onbex.co` absent |

This is a genuine browser-domain divergence, not a REST/GraphQL/MCP/dashboard schema gap. The capture is retained as evidence of why `onbex.co` cannot be a shared tenant suffix. As of 2026-08-08 production leaves `BEX_BASE_DOMAIN` unset, both serving processes reject ordinary registrable domains, and custom domains remain available.

Reproduce the historical unsafe behavior diagnostically:

```sh
PSL_EXPECTED=absent node scripts/static-site-browser-isolation.mjs
```

Gate a replacement suffix before enabling it:

```sh
BEX_HOSTING_SUFFIX=hosting.example node scripts/static-site-browser-isolation.mjs
```

The gate fails until the browser-consumed PSL recognizes the candidate as a private suffix and rejects its parent cookie. Runtime validation provides the matching startup gate.
