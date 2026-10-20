# CLI sweep 615 — maintenance mode is plan-gated: named refusals on both channels

**Verdict: no bug filed.** Same isolated QA session, `bex-canary`, CLI at `f31004f7c`, 2026-10-19. Owned fixture `qa615-web` (`srv-dbbainkgnrlc73bhst9g`, free hello-app), deleted at close: REST 404, zero `qa615` names. The live maintenance page itself could not be exercised — enabling maintenance mode requires a paid plan, which this loop never submits.

- **CLI channel:** `services update --maintenance-mode` on the free service → `400 maintenanceMode requires a paid web service plan`, exit 1; the service kept serving 200 and `maintenanceMode` stayed `{enabled: false, uri: ""}`.
- **REST channel:** `PATCH {serviceDetails:{maintenanceMode:{enabled:true, uri:…}}}` → the same named `400`. No partial state was written.
- **Disable is a safe no-op:** `PATCH …{enabled:false}` on the (already disabled) free service returned 200 — turning maintenance off is not gated, only turning it on.

## Unrun

- The maintenance page/redirect behavior itself (paid plan required).
- Maintenance mode on a service that is already suspended, or enabled before a plan downgrade.
