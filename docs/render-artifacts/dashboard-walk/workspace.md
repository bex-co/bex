# Workspace, projects, and environments

## Page-by-page verdicts

| Page | Live Render evidence | bex evidence | Verdict | Disposition |
| --- | --- | --- | --- | --- |
| Workspace overview | Resource overview and creation entry points | Resource overview and creation entry points | Match for supported resources | Not a gap. |
| Projects | Search plus All/Services/Env Groups filtering over resources | Each Environment card has URL-owned name/id search plus filters for Render's categories and bex's existing Database/Key Value memberships | Match; bex exposes its additional supported resource types | Closed by `w5/m36` on 2026-07-15. `render-walk-projects.png` / `bex-walk-project.png`. |
| Environment detail/settings | Resource grouping and environment settings | Create/rename/delete and Manage resources dialogs, including services and datastores | Match for supported scope | Not a gap. Current capture: `render-walk-environment.png`; shipped bex dialog evidence: `m31-environment-settings.png`. |
| Environment groups list/create | Dedicated list and one-step create with initial variables/files | List/detail/create exist, but local OpenBao absence produced the documented unavailable state; one-step initial contents remain incomplete | Already-owned gap plus configured degradation | No duplicate: initial-content completeness belongs to [w5/m33](../../../.pm/w5/m33/README.md). `render-walk-environment-group-create.png` / `bex-walk-environment-groups.png`. |
| Team | Member table with search | Accepted-member table, accessible identity search, and unchanged role actions; empty/no-match states are distinct | Match | Closed by `w5/m36` on 2026-07-15. `render-walk-workspace-team.png` / `bex-walk-workspace-team-and-audit.png`. |
| Usage/Billing | Accrued charges, included usage, and invoices | Raw usage quantities, month selection, and trends | Deliberate bex extension, not a billing clone | Not a gap. ADR023 owns the intentional model. `render-walk-workspace-usage.png` / `bex-walk-workspace-usage.png`. |
| Audit | Date-range CSV export under workspace compliance settings | Interactive paginated in-app audit table under Security & Compliance | bex is a deliberate superset | Not a gap. `render-walk-workspace-audit.png` / `bex-walk-workspace-team-and-audit.png`. |
| Workspace settings | Workspace identity/settings and danger zone | Equivalent identity/settings and danger zone; Team/Audit live at their existing bex routes | Functional match; placement differs | Not a gap. `render-walk-workspace-settings.png` / `bex-walk-workspace-settings.png`. |

## Classification notes

- The local Env Groups unavailable state is an honest result of running the isolated dashboard without OpenBao, not evidence that the shipped routes are absent.
- Render's Audit page is export-only; bex's searchable table is intentionally ahead, so the existing partial ledger classification is not contradicted.
- Usage measures raw platform quantities and exposes them through APIs by design. It should not be made visually identical to Render Billing.
- Team/Audit route placement and the project Environment-card layout are information architecture choices unless they remove a control.
