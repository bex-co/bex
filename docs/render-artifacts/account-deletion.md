# Account deletion parity — 2026-08-29

## Evidence boundary

- Render's current public OpenAPI (`https://api-docs.render.com/openapi/render-public-api-1.json`) exposes its user read operation but no account-delete operation.
- Render's dashboard places account deletion at the bottom of the user profile/settings surface. Render does not publish a contract for how that action disposes multiple workspaces, creator references, credentials, or retained billing history.
- Ory Kratos's current admin API defines permanent identity deletion at `DELETE /admin/identities/{id}` and a separate all-sessions deletion endpoint. The bex workflow treats both a successful delete and an already-absent identity as convergence.

The UI placement claim is a dated dashboard observation, not a public Render API guarantee. No response-body or multi-workspace behavior is inferred where Render publishes none.

## Parity decision

| behavior | Render evidence | bex |
| --- | --- | --- |
| discoverability | bottom of profile/settings | fifth `/settings` section, **Danger zone** |
| irreversible confirmation | destructive profile action | exact `delete my account` phrase, checked server-side |
| public self-delete API | absent from public OpenAPI | REST + GraphQL over one Core verb |
| MCP | no public operation | intentionally omitted |
| workspace impact preview | not publicly documented | names delete/leave/block workspaces before intent |
| retry/recovery | not publicly documented | durable tombstone and background convergence |

REST + GraphQL are a deliberate bex extension required by ADR008's API-first rule; the dashboard does not become a second control plane. The explicit multi-workspace preview is also a deliberate safety improvement, not claimed Render wire parity.

## Implementation evidence

- Core and adapters: `lego/backend/internal/accounts/`
- durable state: migration `0102_account_deletions`
- onboarding suppression: `lego/backend/internal/api/tenancy.go`
- UI and matched route skeleton: `dashboard/src/features/auth/pages/settings-page/` and `dashboard/src/common/components/route-skeletons.tsx`
- lifecycle contract: [ADR086](../ADR086-account-deletion.md)
- isolated-stack proof procedure: [account-deletion-verify.md](../runbooks/account-deletion-verify.md)
