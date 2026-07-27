# w5 · m54 — Source + Branch pickers (GitHub-App-backed)

**Worker:** worker5 **Goal:** The Build card's Source row becomes editable (switch the connected repo through the existing repo-picker flow) and Branch becomes Render's searchable combobox fed by the repository's real branches via the GitHub App — with free-text fallback whenever the App isn't connected. Lower priority than m50–m53. **Status:** done (2026-07-27)

## Tasks (in order)

| id   | title                                                                    | est | depends_on |
| ---- | ------------------------------------------------------------------------ | --- | ---------- |
| t001 | bex-api: branch-list read for a connected repo (GraphQL, GitHub App) — **DONE** | 45m | —          |
| t002 | Dashboard: searchable Branch combobox with free-text fallback — **DONE** | 45m | t001       |
| t003 | Dashboard: Source row edit affordance (change connected repo) — **DONE** | 45m | t001       |
| t004 | Render parity — cross-surface consistency check — **DONE** | 30m | t002, t003 |
| t005 | Simplify — run `/simplify` over the changed code — **DONE** | 20m | t004       |
| t006 | Test coverage — branch feed, fallback, and repo-switch tests — **DONE** | 30m | t004       |
| t007 | Closeout — verify DoD, mark done, move milestone — **DONE** | 15m | t006       |

## Definition of done

On dev-5 with the GitHub App connected: the Branch row is a searchable combobox listing the repo's actual branches (typeahead filter), selection saves through the existing `setBranch` verb and triggers the rebuild confirm; without the App (or for non-GitHub `https://` repos) the row degrades to today's validated free-text editor. The Source row offers Edit, opens the repo picker (create-wizard component), and switching repos PATCHes `repo` with an explicit rebuild confirmation. Both retire their "conscious divergence" notes in ADR018. Suites green.

## t004 parity walk (2026-07-27)

Live local walk (`yarn local-bex` + `yarn dev:local`) of `eden-cms-v2`: the Build card's **Source** row is now a **disabled combobox** showing the repo URL with an **Edit Source** pencil (was a read-only div), and **Branch** is a disabled combobox with **Edit branch** (t002). Both are Render's searchable-combobox-with-pencil, degrading to free text (allowCustom) with no GitHub connection. Source edit switches `spec.repo` via GraphQL `setRepo` (the shared source verb, rebuild-confirmed); Branch via `setBranch`. Backend unit-tested (`repoBranches`, `githubOwnerRepo`, `ListBranches`); the setRepo mutation reuses `SetSourceAndRegistryCredential`. ADR018 retires the free-text-Branch + read-only-Source divergences. **Outcome: clean.** Snapshot: `.playwright-mcp/m54-source-branch.md`.

## Source + Goal linkage

- **Source:** user request 2026-07-26 — live Render walk 2026-07-26/27: Render's Branch is a search-placeholder combobox and Source has an edit affordance; bex shows Source read-only and edits Branch as free text (w5/m48 recorded the divergence consciously).
- **Goal linkage:** Render parity pillar; upgrades two documented ◐ divergences to ✅ using infrastructure that already exists (GitHub App, `docs/ADR026-github-integration.md`; repo picker from w5/m15; `use-repos.ts`; backend PATCH already accepts `repo`).
- **Expected outcome:** No more typo'd branch names or dead-end Source rows; repo moves (org renames, monorepo splits) don't require service re-creation.
- **Why now:** Same walk, lower urgency — sequenced after m50–m53; kept on the board so the divergence notes have an owner. GitHub-only by standing decision (GitLab/Bitbucket rejected, `.pm/DO_NOT_DO.md`).
- **Render parity task included:** yes — new GraphQL read + UI affordances over the Render-compatible update path.
