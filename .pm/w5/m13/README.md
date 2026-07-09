# w5 · m13 — Dashboard: Build & Deploy settings section (Root Directory)

**Worker:** worker5 **Goal:** a new "Build & Deploy" section on the service Settings tab showing Repo/Branch/Root Directory, wired to the `rootDir` field `w1/m18` adds — so setting a monorepo subdirectory is a dashboard flow instead of a curl/MCP-only capability. **Status:** todo (gated on w1/m18)

## Tasks (in order)

| id   | title                                                                                          | est | depends_on |
| ---- | ------------------------------------------------------------------------------------------------ | --- | ---------- |
| t001 | Capture Render's Settings → Build & Deploy section live (Playwright) as the layout reference      | 25m | —          |
| t002 | New "Build & Deploy" Settings-tab section (Repo/Branch/Root Directory) wired to the `rootDir` GraphQL field from `w1/m18` | 45m | t001       |
| t003 | Save flow: edit → confirm → mutation → rollout, following `w5/m7`'s plan-picker confirm pattern   | 30m | t002       |
| t004 | Live verify: set `rootDir` on a monorepo test app, confirm build succeeds and an out-of-root push doesn't redeploy | 30m | t003       |

## Definition of done

A signed-in user views and edits an App's Root Directory from Settings → Build & Deploy, matching Render's field semantics; saving updates `spec.rootDir` via GraphQL and triggers a rebuild scoped to that subdirectory; verified live against a real monorepo test app.

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w1` (Root Directory topic) 2026-07-09; UI half of `w1/m18`. Follows the Settings-tab pattern established by `w5/m7` (Instance Type section) and fills the "other sections (name/region/build/deploy) are future milestones" gap explicitly noted in `dashboard/src/routes/services.$serviceId.settings.tsx`.
- **Goal linkage:** Render-parity dashboard surface (`dashboard/CLAUDE.md`).
- **Expected outcome:** root-directory configuration for monorepo apps becomes a dashboard flow, not curl/MCP-only.
- **Why now:** sequenced right after `w1/m18` ships the field/API, mirroring this board's repeated API-then-UI ordering (`w1/m8`→`w5/m7`, `w1/m11`→`w1/m11.5`).
- **Render parity: included** — this is a new Settings UI section mirroring Render's own Build & Deploy panel (t001 captures the live reference, the standing Render-parity task verifies drift).
