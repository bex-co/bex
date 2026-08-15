# w4 · m36 — Universal `@` mention search in the /agents composer (Devin/Cursor-style)

**Worker:** worker4 **Goal:** typing `@be` in the /agents composer surfaces the repo `bex-co/bex` (and matching sessions) directly — no `@repos:` category hop required **Status:** done

## Tasks (in order)

| id   | title                                                                                | est | depends_on | status        |
| ---- | ------------------------------------------------------------------------------------ | --- | ---------- | ------------- |
| t001 | mention.ts: merge entities into the top level — universal ranked, grouped options    | 45m | —          | — **DONE**    |
| t002 | Picker UI: grouped sections with headers, top-level loading/empty states, locales    | 30m | t001       | — **DONE**    |
| t003 | Editor/composer integration: keyboard nav across groups, insertion, `@` button, docs | 30m | t002       | — **DONE**    |
| t004 | Reference parity: Devin/Cursor behavior benchmark + serialization contract unchanged | 20m | t003       | — **DONE**    |
| t005 | Simplify: `/simplify` over the changed mention/picker/editor code                    | 20m | t004       | — **DONE**    |
| t006 | Test coverage: pin `@be` → repo surfaced + universal-level edge cases                | 30m | t004       | — **DONE**    |
| t007 | Closeout                                                                             | 15m | t005, t006 | — **DONE**    |

## Definition of done

In the /agents composer, typing `@be` (no category prefix) shows `bex-co/bex` in the dropdown, grouped under a "Repositories" section header, selectable by keyboard and mouse into the same atomic mention badge as today; matching session titles surface the same way under "Sessions". The `@repos:` / `@sessions:` explicit-narrowing tokens still work. The payload sent to `createAgentSession` is byte-identical to before (bare `repo: "owner/name"`, `sessionIds[]` — no prefix leakage). A regression test asserts the bare-`@query` → entity path; `yarn test` and `yarn lint` green in `dashboard/`.

## Source + Goal linkage

- **Source:** user request 2026-08-14 after a Devin `@`-mention research session. Findings: Devin's bare `@` opens a categorized dropdown (Repos/Files/Macros/Playbooks/Skills/Secrets/Sessions) but typing a name after `@` matches entities directly — the category rows are optional narrowing, never a required prefix ([docs.devin.ai/get-started/first-run](https://docs.devin.ai/get-started/first-run), release notes 2025-03-19 "tag `@file_name`" + 2025-10-10 repo badges). Cursor documents the same universal-first model explicitly (most-relevant suggestions first, category rows filter within a type); Linear/Notion are also universal-`@` with grouped sections. bex's current picker is the outlier: `mentionStateFromQuery` (`dashboard/src/features/agent-sessions/lib/mention.ts:55`) only descends into a category when the query literally starts with `repos:`/`sessions:`, and the top level builds rows from **only** the two category entries (`mentionOptions`, `mention.ts:166`), so `@be` scores against `["repos","Repositories"]`/`["sessions","Sessions"]`, matches nothing, and shows "No matches".
- **Goal linkage:** pillar 5 (cloud coding-agent sessions, ADR047 D9) — the /agents composer is the primary human entry point to agent sessions; context attachment friction is front-and-center in every session creation. Matches the AI-native-UX bar the vision doc sets against Devin as the reference product.
- **Expected outcome:** one keystroke sequence (`@` + a few name characters) reaches any repo or prior session; the two-step category ceremony becomes an optional power-user filter. Observable: the DoD regression test, plus a manual dropdown check on `/agents`.
- **Why now:** all prerequisites already exist client-side — repos and sessions are pre-loaded (Apollo `Repos` query + sessions cache), the `matchQuality` fuzzy scorer already ranks the second level, and the backend contract never sees the prefix, so the change concentrates in one pure lib + presentation. Direct user request; small window before more mention types (files, secrets — Devin ships seven) get layered onto the wrong two-level foundation.
- **Render parity note:** the standing parity task is reframed as **reference parity** — Render has no agents composer or `@`-mention surface (this is a bex-native pillar-5 extension; the reference products are Devin and Cursor), and no REST/GraphQL/MCP shape changes: the `createAgentSession` contract is asserted unchanged in t004.

## Reference parity (t004)

- **Benchmark met:** bare `@` opens a categorized list; typing a name after `@` matches entities directly (repos + sessions), with the `@repos:`/`@sessions:` prefixes surviving as optional narrowing — Devin's and Cursor's universal-first model.
- **No wire change:** `composer-document.ts`, `use-agent-session-mutations.ts`, and `new-session-composer.tsx` are untouched; `createAgentSession` still receives a bare `repo: "owner/name"` + `sessionIds[]`. The `repos:`/`session:` tokens never leave the browser. Asserted by the existing + new composer tests (`create.mock.calls[0][0].repo` is bare). No REST/GraphQL/MCP surface.
- **Deliberate divergence:** entities render under **grouped section headers** by type (Linear's documented shape; reusing the existing category labels). Devin's exact merged-vs-grouped layout and its ranking/fuzzy-vs-prefix rules are undocumented publicly, so grouped sections is bex's choice, not a copy — recorded rather than chased.
- **Future mention types** (files, secrets — Devin ships seven) filed as inbox note `w4/033.md`, not implied; the registry stays at `repos | sessions`.

## Closeout

Done 2026-08-14. The `@` picker is universal: `mentionOptions` (`dashboard/src/features/agent-sessions/lib/mention.ts`) scores category rows + repos + sessions against one query at the top level (extracted `rankAndCap`, added `mentionOptionGroup`), the picker renders grouped section headers reusing the existing category labels, and the editor filters already-mentioned sessions at both levels. Empty `@` previews categories + a bounded slice of each entity group; a typed `@be` surfaces `bex-co/bex` directly and `@repos:`/`@sessions:` survive as optional narrowing. No wire change — `composer-document.ts`/mutations/composer untouched.

- **Tests:** 16 new/updated cases (lib universal-level ordering/caps/ranking + `mentionOptionGroup` + `mentionEmptyText` loading; composer one-step repo/session selection, grouped-header preview, selected-session dedupe, category-narrowing retained). Full dashboard suite green (2102) and `yarn lint` clean.
- **Simplify (t005):** three parallel review agents. Reuse + efficiency came back clean; quality found the shared `agentSessionView` test factory (`src/test/mocks/agent-session.ts`) — replaced two hand-rolled session fixtures with it, killing a rotted copy that still carried a phantom `evidence` field. Also deleted a WHAT-narrating comment and added the group-contiguity contract note.
- **Not run:** a live-browser check on `/agents` — the local-bex stub returns no repos and no cluster is available this session (same constraint as m28/m31/m32). The component tests drive the real Tiptap editor + picker through userEvent (typing `@anv`, Enter-inserting the atomic mention, asserting the header), which is the highest-fidelity verification available here.
