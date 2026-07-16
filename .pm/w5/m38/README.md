# w5 · m38 — Environment IP rules UI: description-carrying entries

**Worker:** worker5 **Goal:** the environment ACL editor stops being the odd one out — it reads and writes the backend's description-carrying `ipAllowListEntries` form (like the datastore access panels already do, and like Render's `cidrBlockAndDescription` shape), so an environment rule can carry a human label. **Status:** todo

## Tasks (in order)

| id   | title                                                                                        | est | depends_on |
| ---- | ---------------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Swap the env ACL query/mutation to `ipAllowListEntries` (`{cidrBlock, description}`) + hook     | 30m | —          |
| t002 | Editor UI: per-rule description input + display, locales (en/zh), matching the datastore panel  | 45m | t001       |
| t003 | Render parity — env-rule editing vs Render's protected-environment UX                           | 20m | t002       |
| t004 | Simplify — `/simplify` over the milestone's diff                                                | 15m | t003       |
| t005 | Test coverage — meaningful tests for the shipped behavior                                       | 30m | t003       |
| t006 | Closeout — move to `done/` when the DoD holds                                                   | 15m | t005       |

## Definition of done

In the dashboard, an environment IP rule can be added with a description; the description round-trips (visible on re-query and after reload), rules without descriptions still work, and existing plain-string rules render unharmed. The editor's read/write path uses `ipAllowListEntries` end to end; component tests cover add/edit/remove with and without descriptions.

## Source + Goal linkage

- **Source:** `/pm-brainstorm` round 21, 2026-07-15 — dashboard-gap mine G3: `environment-settings-dialog.tsx` + `use-set-environment-acl.ts` use the plain CIDR-string form (`SetEnvironmentACL($ipAllowList: [String!])`, `environments.graphql:63-75`) while the backend ships `ipAllowListEntries` with `{cidrBlock, description}` (`environments/graphql.go:46,189`, `gqlutil.IPAllowEntryInputType`) and the datastore editors already render descriptions (`access-control-panel.tsx:49-91`).
- **Goal linkage:** dashboard parity with Render's protected-environment UX (`docs/render-artifacts/protected-environments.md`); w4/m24 built description persistence precisely so surfaces stop dropping what they accept.
- **Expected outcome:** environment rules match the datastore panels' fidelity; no accepted-but-invisible field on the env ACL surface.
- **Why now:** enforcement for these rules went live today (w4/m28) — rules users can't label are about to multiply; the backend form and a copyable UI pattern both already exist.
- **Render parity:** included (t003) — UI-surface feature work.
