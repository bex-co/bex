# w5 · m51 — Build Command for all native-runtime services + root-dir command prefix

**Worker:** worker5 **Goal:** The settings page shows an editable Build Command row for every repo-backed native-runtime service (today it exists only for static sites), and every command input carries Render's root-dir-aware prompt prefix (`<rootDir>/ $`) so commands read as relative to the root directory. **Status:** todo

## Tasks (in order)

| id   | title                                                                        | est | depends_on |
| ---- | ----------------------------------------------------------------------------- | --- | ---------- |
| t001 | Un-gate the Build Command row for all native-runtime repo-backed services     | 30m | —          |
| t002 | Root-dir prompt prefix inside Build/Pre-Deploy/Start Command inputs           | 30m | t001       |
| t003 | Render parity — cross-surface consistency check                               | 30m | t002       |
| t004 | Simplify — run `/simplify` over the changed code                              | 20m | t003       |
| t005 | Test coverage — gating + prefix behavior tests                                | 30m | t003       |
| t006 | Closeout — verify DoD, mark done, move milestone                              | 15m | t005       |

## Definition of done

On dev-5: a repo-backed **node web service** shows a Build Command row in Build & Deploy, edits save through the existing `setBuildCommand` mutation, and the change triggers the documented rebuild path; a docker-runtime service shows Dockerfile Path instead (no Build Command row); with `rootDir` set to `backend`, the Build, Pre-Deploy, and Start Command inputs each render a `backend/ $` prefix (plain `$` when rootDir is unset), matching Render's live rendering. Dashboard suite green.

## Source + Goal linkage

- **Source:** user request 2026-07-26 ("the build command is missing") — confirmed on the live bex settings page for the node web service `srv-d9bkcspg9s7c73d0n8ug` (no Build Command row) vs Render's page (Build Command `yarn` with a `backend/ $` prefix, root dir `backend`). The backend already supports `spec.buildCommand` + GraphQL `setBuildCommand` + REST PATCH + MCP for every type (w7/m41 shipped the static-site UI only).
- **Goal linkage:** Render parity pillar (`docs/ADR018-render-parity.md`); closes a pure-UI gap over an already-shipped full-stack verb.
- **Expected outcome:** Users of native-runtime services can see and change how their app builds without leaving the dashboard; the prefix makes root-dir/command coupling legible exactly as on Render.
- **Why now:** Directly user-reported against the production dashboard; smallest-possible surface change because the whole backend path already exists. Builds on m50's shared row (prefix slot) — sequence after m50/t001.
- **Render parity task included:** yes — user-facing UI change over a Render-compatible verb; the check walks REST/GraphQL/MCP + UI for consistent buildCommand semantics per type.
