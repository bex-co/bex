# w5 · m30 — Dockerfile path + start command in Build & Deploy settings

**Worker:** worker5 **Goal:** The dashboard exposes `dockerfilePath` (settable nowhere in the UI today) and makes `startCommand` editable post-create, closing the two dashboard holes w6/m21 left after shipping both fields across every non-UI surface. **Status:** todo

## Tasks (in order)

| id   | title                                                            | est | depends_on |
| ---- | ---------------------------------------------------------------- | --- | ---------- |
| t001 | `startCommand` inline-edit in Settings → Build & Deploy          | 40m | —          |
| t002 | `dockerfilePath` inline-edit in the same section                 | 30m | t001       |
| t003 | `dockerfilePath` in the create wizard's docker-runtime branch    | 30m | —          |
| t004 | Render parity                                                    | 30m | t002, t003 |
| t005 | Simplify                                                         | 30m | t004       |
| t006 | Test coverage                                                    | 40m | t004       |
| t007 | Closeout                                                         | 15m | t006       |

## Definition of done

In the dashboard Settings → Build & Deploy section, `startCommand` and `dockerfilePath` each have an inline-edit control (pencil → input → confirm, the `use-root-dir.ts` pattern) that persists through save/reload; the service-create wizard's `runtime: docker` branch offers a `dockerfilePath` field alongside the existing `startCommand`; `yarn test` green. No backend change — both fields already round-trip on REST/GraphQL/MCP (w6/m21).

## Source + Goal linkage

- **Source:** `.pm/w6/015.md` (filed by `w6/m21`'s t005 Render-parity check, 2026-07-14), promoted via `/pm-brainstorm more milestones for each worker` round 5, 2026-07-14 — the last unowned, non-anti-goal parity gap in `docs/ADR018-render-parity.md`.
- **Goal linkage:** Render parity (service-create-fields + Build & Deploy settings row); w5's dashboard charter. Closes the create-only asymmetry `startCommand` had vs its siblings (`rootDir`/`preDeployCommand`/`autoDeploy`, each already inline-editable) and the total UI absence of `dockerfilePath`.
- **Expected outcome:** a user deploying a Dockerfile at a non-default path, or changing a start command after create, has a dashboard path — not just bex.yml/REST/GraphQL/MCP.
- **Why now:** the backend/operator work landed in `w6/m21` (2026-07-14) leaving only these two UI holes, and the exact inline-edit pattern to copy (`use-root-dir.ts`/`root-dir.graphql`) is established; the parity gap-well is otherwise dry (round-5 census: every other ADR018 ✖/◐ is owned or a deliberate non-goal). Render parity task included — pure UI surface change over an existing backend.
