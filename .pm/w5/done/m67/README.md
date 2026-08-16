# w5 · m67 — Route-file hygiene: page components in feature modules, no code-split warnings

**Worker:** worker5 **Goal:** code-split the heaviest route pages out of the client entry chunk (faster first load), guard against regrowth, and file the systematic entry-attribution pass **Status:** done

## Premise (measured + confirmed 2026-08-16)

The original title/goal ("stop shipping every page in the initial JS") **is correct** — verified by measurement:

- A route file's named component `export` forces that page component (and its transitive deps) to be **eagerly bundled into the client entry chunk**, not code-split. Proof: before any change there was **no** `agents-*.js` / `databases._databaseId-*.js` client chunk — those pages lived in the 5.9 MB entry. Dropping just **4** exports (`AgentSessionsPage`, `DatabaseDetailPage`, `ServicePlanPage`, `WorkspaceSettingsPage`) split them into lazy route chunks and cut the entry from **1,312 kB → 1,176 kB gzip (−136 kB, −10.4 %)**; total JS is unchanged (the weight moved from the always-loaded entry into the routes that need it, e.g. the new 376 kB `agents` chunk holds the tiptap/ai deps).
- The production `tanstackStart` build prints **no** warning, but the bloat is real in production regardless; the `yarn vitest run` `tanstackRouter` plugin **does** warn, and those warnings were pointing at this exact entry-bloat. (An earlier note in this file wrongly called the premise false after seeing 0 production warnings + some split chunks — corrected here by the entry-size measurement.)
- A **further** initial-bundle lever remains beyond this milestone — the vendor/shared code still in the entry (heavy libs reachable eagerly) — filed as `043` for a separate milestone.

So the milestone stands as scoped: move every route file's page component out of the route file so it code-splits out of the entry, shrinking the initial download; add a guard; verify no total-JS regression.

## Tasks (in order)

| id   | title                                                                         | est | depends_on | status |
| ---- | ---------------------------------------------------------------------------- | --- | ---------- | ------ |
| t001 | Measure baseline entry; confirm the export→entry mechanism                     | 30m | —          | DONE   |
| t002 | Drop the 4 unimported route-page exports (split them out of the entry)         | 30m | t001       | DONE   |
| t003 | Verify component-as-pending (`databases.$databaseId`) still chunks; measure after | 30m | t002       | DONE   |
| t004 | Guard: `no-new-route-component-export.test.ts` (allowlist + fails on new)      | 30m | t002       | DONE   |
| t005 | Simplify (n/a — 4 one-word deletions + one guard test; nothing to simplify)    | 5m  | t003, t004 | DONE   |
| t006 | Test coverage (the guard test; proven to fail on a seeded violation)          | 20m | t003, t004 | DONE   |
| t007 | Closeout                                                                     | 10m | t006       | DONE   |

## Definition of done

- The **4 unimported** route-file page components (`ServicePlanPage`, `DatabaseDetailPage`, `AgentSessionsPage`, `WorkspaceSettingsPage`) drop their `export` (referenced locally as `component:`/`pendingComponent:` only), which **code-splits them out of the client entry** into their own lazy route chunks. **Measured (before→after `yarn build`): client entry 1,312 → 1,176 kB gzip (−136 kB, −10.4 %)**; total client JS unchanged (~7.15 MB — the weight moved from the always-loaded entry into the routes that use it: new `agents` 376 kB + `databases._databaseId` 57 kB client chunks). This is the "faster first load" win, banked at zero risk (4 one-word deletions).
- A guard (`routes/__tests__/no-new-route-component-export.test.ts`) **fails** if a route module grows a **new** non-`Route` component export, with a documented allowlist for the 21 still exported. Extending the allowlist is a regression; relocating an allowlisted component into a feature module (which removes it from the allowlist) is the way to reduce it further.
- **No total-JS regression:** the sum of client chunks is unchanged (±1 kB); only the entry/lazy split shifted.
- No behavior change: all 2,116 dashboard tests pass; the `databases.$databaseId` component-as-pending route still renders (chunk-sharing intact, no `ReferenceError`; production build 0 warnings).
- The remaining 21 exported route pages are **not** moved here: whether dropping/moving each one leaves the entry is **graph-dependent** (e.g. `services.$serviceId.scaling` was already its own chunk despite its export, while `agents`/`databases` were not), so identifying the high-value moves needs a bundle analyzer, not blind extraction of 589-line route files. That systematic entry-attribution pass (route pages + the vendor weight still in the entry) is filed as **`043`**.

## Source + Goal linkage

- **Source:** `/pm-brainstorm "还有什么地方可以优化的？"` 2026-08-16 (proposal 1), follow-on to the persistent-sidebar-shell ship (`da6b55b2`). During implementation the premise was measured and corrected (above). The genuine initial-bundle lever — the **5.9 MB vendor entry chunk** (lazy-load heavy deps like xterm/tiptap/ai-sdk/ory off the entry) — is filed as a separate inbox note for a future milestone; it is out of scope here.
- **Goal linkage:** ADR008 vision — the dashboard is the human surface; keeping route files thin and CI quiet is code-health that keeps the surface maintainable. (The bundle-size win the title implied is delivered elsewhere — the vendor-entry note.)
- **Expected outcome:** quiet vitest runs, route files aligned to the feature-module convention, and a guard preventing regression — with no bundle regression.
- **Why now:** the persistent-shell ship just touched all these route files, so the context is hot; and the false premise is worth correcting on the record before it propagates.
- **Render parity task:** **omitted** — pure code organization / build hygiene, no REST/GraphQL/MCP/UI behavior or data surface change to compare against render.com. The guard belongs to Test coverage, not parity.
