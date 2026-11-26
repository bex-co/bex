# Fixture ledger

A fixture shaped like `docs/render-artifacts/dashboard-operations.md`.

| Flag | Called from | Disposition | Pointer |
| --- | --- | --- | --- |
| `okta-app-available` | `Settings` | flagged | SSO is a non-goal (ADR018 § Identity, workspaces & account: SSO / SAML · SCIM (—)) |
| `postgres-parameter-overrides` | `index` | flagged | Reopen `w2/066` D2=A: the overrides card |
| `share-postgres` | `index` | flagged | Legacy sharing; nothing to reopen (ADR018 § Identity, workspaces & account: Workspace members & roles) |
| `skip-banner` | `index` | flagged | Compare bex's banner (ADR018 § Billing & invoicing: Unpaid invoice, suspension and recovery) |

## Operations

### Services

| Operation | Kind | Disposition | Pointer |
| --- | --- | --- | --- |
| `server` | query | matched | ADR018 § Services & lifecycle: Get service |
| `serviceEvents` | subscription | matched | ADR018 § Services & lifecycle: Service events / activity feed |
| `statusQuery` | query | matched | ADR018 § Services & lifecycle: Get service |
| `updateServer` | mutation | matched | ADR018 § Services & lifecycle: Get service |

### Deploys and billing

| Operation | Kind | Disposition | Pointer |
| --- | --- | --- | --- |
| `addPromotion` | mutation | non-goal | DO_NOT_DO: ADR071 tenant billing credits |
| `ownerFeatureFlags` | query | n/a | Render's feature-flag plumbing |
| `rollbackComparison` | query | gap | `w1/m646` |
