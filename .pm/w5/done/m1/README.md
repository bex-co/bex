# w5 · m1 — Scaffold dashboard from beancount-dashboard

**Worker:** worker5 **Goal:** Stand up `dashboard/` at the repo root as an empty-but-working TanStack Start + Apollo + shadcn app shell — a clean base for wiring to `bex-api`'s GraphQL later, with none of beancount's domain code left behind. **Status:** done (all 7 tasks done)

## Tasks (in order)

| id   | title                                                               | est | depends_on | |
| ---- | -------------------------------------------------------------------- | --- | ---------- | --- |
| t001 | Copy beancount-dashboard into `dashboard/`                            | 20m | —          | **DONE** |
| t002 | Strip beancount domain features, and docs                            | 45m | t001       | **DONE** |
| t003 | Rebrand identifiers (package name, README, CLAUDE.md, env, titles)    | 30m | t002       | **DONE** |
| t004 | Add a minimal sample route using the existing UI kit                  | 30m | t003       | **DONE** |
| t005 | Verify install/typecheck/lint/build/dev all boot clean                | 30m | t004       | **DONE** |
| t006 | Simplify                                                              | 20m | t005       | **DONE** |
| t007 | Test coverage                                                        | 30m | t005       | **DONE** |

## Definition of done

`dashboard/` exists at the bex5 repo root as a TanStack Start app with beancount's `src/features/*`, `src/graphql/{query,mutation}`, `api-gateway.graphql`, and beancount-specific docs removed — but its Apollo Client + `graphql-codegen` **plumbing retained** (this dashboard is meant to become a client of `bex-api`'s GraphQL, so the wiring mechanism stays, only beancount's schema/operations content goes); `package.json`, `README.md`, `CLAUDE.md`, `.env.example` and page titles rebranded to bex; one sample route renders placeholder/sample content using the existing shadcn/Radix component kit; and `yarn install && yarn typecheck && yarn lint && yarn build` all succeed with `grep -ril beancount dashboard/ --exclude-dir=node_modules` returning nothing.

## Source + Goal linkage

- **Source:** user request 2026-07-05 — "copy /projects/web-beancount/beancount-dashboard to dashboard and then clean up beancount specific things to make it an almost empty dashboard project with sample contents" — plus this session's earlier discussion recommending a top-level `dashboard/` directory as the client of `bex-api`'s GraphQL adapter.
- **Goal linkage:** `docs/vision.md` pillar 1 — `bex-api` REST/GraphQL is explicitly "Render dashboard compatible" (`docs/bex-api.md`); this scaffolds the dashboard that adapter exists to serve. Must respect pillar 1's API-first rule ("No dashboard-only features, ever") — the dashboard is a client of actions already exposed via REST/GraphQL/MCP, never a source of dashboard-exclusive capability.
- **Expected outcome:** an empty-but-running dashboard app in-repo (`yarn dev` shows a sample page), ready for a future milestone to point its Apollo client at `bex-api`'s `/graphql` instead of beancount's `api-gateway.graphql` schema.
- **Why now:** cheapest to strip a mature, already-battle-tested TanStack Start + Apollo + shadcn stack (`beancount-dashboard`) down to a clean shell before any bex-specific code gets layered on top of it.
