# w2 · m97 — Blueprint preview names the reason a fetch failed

**Worker:** worker2 **Goal:** the Blueprint preview tells an author what to fix. Every fetch failure carries a coded `reason` on REST, GraphQL and MCP, the dashboard picks its title and body from that reason, and no raw `github:` error text or commit SHA reaches any surface. The existing-Blueprint sync view and the auto-sync worker classify the same failures the same way. **Status:** todo

## Tasks (in order)

| id   | title                                                                                                                                 | est | depends_on |
| ---- | ------------------------------------------------------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | `PreviewBlueprint` classifies every fetch failure into a coded `reason` on REST, GraphQL and MCP; `error` becomes human-readable      | 40m | —          |
| t002 | The dashboard picks title and body per reason, shows `invalid_path` beside the path field, and offers Retry only when retryable       | 30m | t001       |
| t003 | The existing-Blueprint sync view and the auto-sync worker reuse the classifier                                                        | 20m | t001       |
| t004 | Render parity                                                                                                                         | 15m | t002, t003 |
| t005 | Simplify                                                                                                                              | 15m | t004       |
| t006 | Test coverage                                                                                                                         | 30m | t004       |
| t007 | Closeout                                                                                                                              | 10m | t006       |

## Definition of done

Run each probe on the production dashboard at `/blueprints/new`, signed in to workspace `bex`, with repository `bex-co/bex` and branch `main` unless the row says otherwise. Replaying the page's own `BlueprintPreview` GraphQL query with changed variables is acceptable for the wire checks; the title and body checks are read from the rendered page.

| input                                                        | expected title                                                          | expected body / behavior                                                                  |
| ------------------------------------------------------------ | ----------------------------------------------------------------------- | ----------------------------------------------------------------------------------------- |
| path `render.yaml` at the root, where none exists            | Blueprint file not found                                                | names the path and branch; no `github:` text, no commit SHA                               |
| path `examples/nope/render.yaml`                             | Blueprint file not found                                                | same shape with that path                                                                 |
| branch `qa-no-such-branch`                                   | Branch not found                                                        | names the branch; no `unexpected status 404`                                              |
| repo `https://github.com/bex-co/qa-no-such-repo.git`         | Repository not found, or bex's GitHub app can't access it               | one combined reason; nothing distinguishes missing from private                           |
| path `examples/hello-go`                                     | not rendered as "not found"                                             | an invalid-path message beside the path field (`.yaml`/`.yml` required)                   |
| path `../render.yaml` and `/render.yaml`                     | not rendered as "not found"                                             | an invalid-path message beside the path field (clean repository-relative path required)   |
| control: `examples/hello-go/render.yaml`                     | Blueprint file parsed successfully                                      | `found: true`; Deploy Blueprint enabled                                                   |
| control: `examples/whoami-app.yaml`                          | Blueprint file has errors                                               | Deploy Blueprint stays disabled                                                           |

Across every row above, the wire `blueprintPreview` carries `reason` set to one of the eight codes, `error` contains no `github:` prefix and no 40-hex SHA, and "Deploy Blueprint" is disabled for every failure row. A Retry control appears only for `rate_limited` and `unavailable`.

## Source + Goal linkage

- **Source:** `w1/097` (live `/qa-find-bugs` 2026-09-14 pass 26, journey 13), absorbed by `/pm-brainstorm for w1` on 2026-09-15 and materialized here per the user's `/pm for all for w2` direction. The note is retired at `.pm/w1/done/097.md`.
- **Goal linkage:** pillar 1 (Render-compatible surfaces): `docs/ADR018-render-parity.md` row 170 still marks Blueprint / `render.yaml` IaC as ◐ on every surface, and the preview's error contract is part of what keeps it there. Pillar 2 (machine-readable state): a coded reason is something an agent can branch on; an internal error string is not.
- **Expected outcome:** at the review step a Blueprint author can tell whether the path, the branch, or the repository is wrong, and API clients receive a stable `reason` instead of parsing prose. The same classification governs manual sync and auto-sync, so the leak is closed in three places at once.
- **Why now:** the classifier is one function shared by preview, sync, and the auto-sync worker; building it once now is cheaper than patching three call sites later. The current wire also carries the commit SHA `discoverBlueprintFile` pinned, which is internal state that has no business on the preview surface.
- **Render parity:** included (t004). The change adds a field to the `BlueprintPreview` shape on REST, GraphQL and MCP and changes dashboard copy, so all four surfaces must agree. Render's own preview copy for these failure cases was not captured during filing; t004 captures it and records any drift. Missing-repo and no-access stay one combined reason on purpose: GitHub returns 404 for a private repository the installation cannot see, and splitting them would turn the preview into a private-repo existence oracle.
