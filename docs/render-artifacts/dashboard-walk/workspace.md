# Workspace, projects, and environments

## Page-by-page verdicts

| Page | Live Render evidence | bex evidence | Verdict | Disposition |
| --- | --- | --- | --- | --- |
| Workspace overview | Resource overview and creation entry points | Resource overview and creation entry points | Match for supported resources | Not a gap. |
| Projects | One selected Environment; counted All/Services/Env Groups filters; Name/Status/Runtime/Region/Updated rows; contextual New service; selection + Move | One URL-selected Environment (plus truthful Unassigned access); counted filters retain Database/Key Value; the same metadata columns; contextual New Service; mixed-kind selection + Move | Match by implementation and regression coverage; deployed populated re-walk still pending | Initial search/filter gap closed by `w5/m36`; `w5/m41` implementation completed on 2026-07-17 but remains open for its deployed dev-2 verification. `render-walk-projects.png` / `bex-walk-project.png`; populated comparison contract below. |
| Environment detail/settings | Resource grouping and environment settings | Create/rename/delete and Manage resources dialogs, including services and datastores | Match for supported scope | Not a gap. Current capture: `render-walk-environment.png`; shipped bex dialog evidence: `m31-environment-settings.png`. |
| Environment groups list/create | Dedicated list and one-step create with initial variables/files | List/detail/create exist, but local OpenBao absence produced the documented unavailable state; one-step initial contents remain incomplete | Already-owned gap plus configured degradation | No duplicate: initial-content completeness belongs to [w5/m33](../../../.pm/w5/m33/README.md). `render-walk-environment-group-create.png` / `bex-walk-environment-groups.png`. |
| Team | Member table with search, seat usage, pending-invite management | Accepted-member table with identity search + per-member 2FA badge, "X of Y seats" from the API, pending invites with resend/revoke — now under **Workspace settings**, matching Render's placement | Match, including placement | Closed by `w5/m36` (search/states) and `w1/m33` on 2026-07-16 (seats, 2FA, resend, workspace-settings placement). `render-walk-workspace-team.png` / `bex-walk-workspace-team-and-audit.png` (pre-move capture). |
| Usage/Billing | Accrued charges, included usage, and invoices | Raw usage quantities, month selection, and trends | Deliberate bex extension, not a billing clone | Not a gap. ADR023 owns the intentional model. `render-walk-workspace-usage.png` / `bex-walk-workspace-usage.png`. |
| Audit | Date-range CSV export under workspace compliance settings | Interactive paginated in-app audit table under Security & Compliance | bex is a deliberate superset | Not a gap. `render-walk-workspace-audit.png` / `bex-walk-workspace-team-and-audit.png`. |
| Workspace settings | Workspace identity/settings, Team Members, and danger zone | Identity/settings, the Team panel (moved here by `w1/m33`), and danger zone; Audit stays under account Security & Compliance (a deliberate superset — see Audit row) | Match; only Audit placement differs, in bex's favor | `render-walk-workspace-settings.png` / `bex-walk-workspace-settings.png` (pre-move capture). |

## Classification notes

- The local Env Groups unavailable state is an honest result of running the isolated dashboard without OpenBao, not evidence that the shipped routes are absent.
- Render's Audit page is export-only; bex's searchable table is intentionally ahead, so the existing partial ledger classification is not contradicted.
- Usage measures raw platform quantities and exposes them through APIs by design. It should not be made visually identical to Render Billing.
- Audit route placement and the project Environment-card layout are information architecture choices unless they remove a control. (Team placement was resolved to Render's own — workspace settings — by `w1/m33`, superseding the earlier IA-choice note for that row.)

## Populated Project comparison — 2026-07-17

The earlier walk used a sparse project and therefore established search/filter reachability but could not expose the populated operating controls. A second authenticated, redacted comparison used a populated Render Project and the dev bex Project without retaining tenant names, resource ids, emails, or credentials.

### Render contract pinned by the populated view

- The selected Environment was the single operating context. Its table showed counted **All**, **Services**, and **Env Groups** categories, search, one row checkbox per service, a selected-count **Move** toolbar, **New service**, **Add environment**, and `Name · Status · Runtime · Region · Updated` metadata. It did not stack every Environment table or repeat the same resources in a second project-wide table.
- Render's public [Projects and environments documentation](https://render.com/docs/projects) confirms that a Project resource belongs to one Environment, that an Environment's empty state offers both create and move-existing paths, that service creation can receive Project/Environment context, and that the individual `••• → Move` action targets another Environment. The same document defines bulk Move from the workspace service list: select services, click **Move**, and choose an Environment.
- Render's captured Project selection was service-only (`N services selected`) and all populated comparator rows were services. Render therefore supplied no mixed-resource rule to copy. bex deliberately applies the same Environment target to every resource kind its Project surface already supports; each kind uses its server-authoritative full-replace Environment verb, preserves unrelated target members, and reports/retries a failed kind independently.
- Runtime is the service runtime/product shown by Render; Region is its resource placement (static sites are described as global in Render's [regions documentation](https://render.com/docs/regions)); Updated is the resource mutation timestamp. bex uses its actual list facts instead: service runtime, datastore product/version, explicit `BEX_REGION`, and `updatedAt`. Missing facts render as unavailable; `createdAt` is never relabeled as Updated.
- Render's public documentation does not specify selection persistence across search changes or copied-URL behavior. bex keeps selected rows while filtering the same Environment, clears successful Move rows while retaining failed rows, and remounts/clears selection when the URL-selected Environment changes. Search/type/Environment state remains URL-owned; a missing or deleted Environment id is replaced with the deterministic first available Environment (or Unassigned).

### bex result

`w5/m41` replaces the stacked Environment cards plus duplicate all-resources table with one selected view, while retaining Environment create/rename/delete/settings/Manage resources and project-only **Unassigned** access. The contextual New Service link validates and preselects the Project/Environment in the existing wizard. Regression coverage exercises counts, URL fallback, unassigned access, all four row kinds, missing metadata, distinct created/updated timestamps, contextual create validation, select-one/select-visible-all, unrelated-member preservation, and mixed-kind partial failure.

### Stateful local Chromium evidence — 2026-07-17

The repository's local bex/Kratos stand-in now seeds a Project with two Environments, two assigned services, Postgres, Key Value, an environment group, and unassigned service/Key Value rows. Chromium exercised the actual dashboard SSR/client application against that stateful API:

- Production rendered one four-row table with accurate `All (4) · Services (1) · Databases (1) · Key Values (1) · Env Groups (1)` counts and `Name · Status · Runtime · Region · Updated` columns. Service, Postgres, and Key Value rows showed their seeded authoritative facts; the environment group showed unavailable Runtime/Region without fabricated values.
- Search wrote `q=orders` while retaining the pre-search counts; the Databases filter wrote `kind=databases`. The contextual action opened `/services/new?projectId=…&environmentId=…`, displayed the seeded Project/Environment, and a browser-created service submitted that `environmentId` once and appeared in the target Environment's server state.
- Select-visible-all produced `Selected: 4`. Moving the service, Postgres, Key Value, and environment group to Staging emptied Production; a fresh navigation to Staging showed all four moved rows plus Staging's unrelated existing service, proving both persistence and preservation.
- A direct `?env=unassigned` navigation showed the two project-only rows and truthful zero/nonzero category counts. This pass exposed and fixed a load-order race that had canonicalized that copied URL before Project rows arrived; regression coverage now pins explicit Unassigned URLs while rows are transiently empty.

This is stateful real-browser evidence for the implementation, not the milestone's required deployed-stack proof. The dev-2 populated re-walk remains pending until the uncommitted Service GraphQL schema addition and dashboard changes are shipped and deployed; no live-parity claim is inferred from the local stand-in.
