# CLI sweep 616 — `renderSubdomainPolicy`: disabling is refused until a custom domain exists

**Verdict: no bug filed.** Same isolated QA session, `bex-canary`, CLI at `f31004f7c`, 2026-10-19. Owned fixture `qa616-web` (`srv-dbbakpabpdss73ci0olg`, free hello-app), deleted at close: REST 404, zero `qa616` names. REST boundary (no CLI flag at this pin).

- **The guard fires live (sweep 564's contract, now enforcement-checked):** `PATCH {renderSubdomainPolicy: "disabled"}` on a service whose only address is the platform subdomain → `400 the platform subdomain can't be disabled until a custom domain is verified, since the service would have no address left`. The URL kept serving 200 throughout; no partial state.
- **Invalid value:** `"banana"` → `400 renderSubdomainPolicy must be enabled or disabled`.
- **Re-enable is a no-op success:** `PATCH … "enabled"` → 200, policy reads `enabled`, URL uninterrupted.
- The actual subdomain shutdown could not be exercised: it needs a verified custom domain, and this loop controls no DNS.

## Unrun

- Disable with a verified custom domain present (URL goes dark, custom domain keeps serving).
- Policy change while the service is suspended.
