# Render: what decides an auto-deploy (skip phrases, Root Directory, Build Filters)

Captured 2026-11-16 for `w7/m428` from Render's public documentation. No live Render deploy was run: a capture needs a Render account, which bex does not have yet (`w10/blocked/002`). Cells the pages do not settle in one sentence are marked **inferred**, with the sentences they rest on.

## Sources

| Source | Read | What it settles |
| --- | --- | --- |
| [render.com/docs/deploys](https://render.com/docs/deploys) § Skipping an auto-deploy | 2026-11-16 | The six skip phrases |
| [render.com/docs/monorepo-support](https://render.com/docs/monorepo-support) | 2026-11-16 | Root Directory, included and ignored paths, their precedence, previews |
| [render.com/docs/blueprint-spec](https://render.com/docs/blueprint-spec) (`rootDir`, `buildFilter`) | 2026-11-16 | The same rules for a Blueprint-declared service |

## Render's sentences

Skip phrases (deploys page):

- "The skip phrase is one of `[skip render]` or `[render skip]`. You can also replace `render` with one of the following:" `deploy`, `cd`. That makes six phrases.
- "When an auto-deploy is skipped, a corresponding entry appears on your service's Events page".
- The page says nothing about letter case, where in the message the phrase sits, or which commit of a push is read. bex keeps its earlier matching (since w4/m162's events work): the phrase may sit anywhere in the push's head commit message, in any letter case, and only the exact bracketed spelling counts (`[skip-render]`, `skip render` and `[skip ci]` do not).

Root Directory and Build Filters (monorepo-support page):

- **R0** "By default, Render automatically deploys your service whenever you push any changes to its linked Git branch."
- **R1** "If you set a root directory for your service, Render only triggers an autodeploy if your changes affect files anywhere under that directory."
- **F1** "Changes that match an included path will trigger an autodeploy, unless those files also match an ignored path."
- **F2** "If you specify at least one included path, all non-matching paths are automatically ignored (you don't need to specify them as ignored paths)."
- **F3** "If you don't specify any included paths, all file changes trigger an autodeploy unless they match an ignored path."
- **F4** "Changes that match an ignored path will not trigger an autodeploy, even if those files also match an included path. In other words, ignoring a path takes precedence over including it."
- **F5** "Build filter paths are always relative to your repository root, even if you've set a different root directory. This means your build filters can include paths from other directories in your repo."
- **P1** (previews) "If you open a pull request that only modifies ignored files for a service, Render skips creating a preview instance for that pull request. A file might be ignored because it's outside your service's root directory or because it matches one of your build filter's ignored paths."
- **P2** (FAQ, preview environments) "Render only creates a preview environment if the files changed in a pull request match the root directory or build filter paths for at least one service."
- **B1** (FAQ) "Changes to a Blueprint's YAML file (usually render.yaml) are always processed regardless of your build filters. Blueprint syncs are also unaffected by build filters."
- **M1** (FAQ) "If you trigger a manual deploy or update your service's configuration, Render always proceeds with the deploy regardless of your build filters."

Blueprint spec: `rootDir` "Changes to files outside the root directory do not trigger a build for the service." `buildFilter` paths "are always relative to the repo's root directory."

## The rule bex applies

Each changed file of the push is judged on its own. A file **triggers** when no ignored path matches it (F4) and:

- included paths are set, and one of them matches it, wherever it lies (F1, F2, F5); or
- no included paths are set, and it lies under the Root Directory, or no Root Directory is set (R1, F3).

The push deploys when at least one changed file triggers. A push whose payload carries no changed paths at all deploys: there is nothing to filter on (a bex rule, unchanged since w1/m18; Render does not describe such a payload). A skip phrase in the head commit's message is checked first and skips the push whatever its files.

When the push does not deploy, the `commit_ignored` event's reason is:

- `skip_phrase` for a skip phrase;
- `root_directory` when no included paths are set and no changed file lies under the Root Directory;
- `build_filter` otherwise (included paths set and none matched, or every candidate file ignored).

The code is `pathsIgnoredReason` in `lego/backend/internal/apps/webhook.go`, and `TestPathsIgnoredReason` walks every cell below.

## Decision table

"Under root" means under the service's Root Directory; with no Root Directory set, every file is under it. "Included" means the file matches an included path; with no included paths set, every file counts as not included. Ignored means it matches an ignored path. Each cell is a push whose changed files all fall in its column; "n/a" marks a column no file can fall in for that row.

| Row | Root Directory | Included paths | Ignored paths | Under root, included | Under root, not included | Outside root, included | Outside root, not included | Ignored |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| 1 | unset | unset | unset | n/a | deploys (stated, R0) | n/a | n/a | n/a |
| 2 | unset | unset | set | n/a | deploys (stated, F3) | n/a | n/a | `build_filter` (stated, F3 + F4) |
| 3 | unset | set | either | deploys (stated, F1) | `build_filter` (stated, F2) | n/a | n/a | `build_filter` (stated, F4) |
| 4 | set | unset | unset | n/a | deploys (stated, R1) | n/a | `root_directory` (stated, R1) | n/a |
| 5 | set | unset | set | n/a | deploys (stated, R1 + F3) | n/a | `root_directory` (stated, R1) | under root `build_filter` (stated, F4); outside it `root_directory` (stated, R1) |
| 6 | set | set | either | deploys (stated, F1) | `build_filter` (**inferred**, A) | deploys (**inferred**, B) | `build_filter` (stated, F2 + R1) | `build_filter` (stated, F4) |

A named reason means the push does not deploy and records `commit_ignored` with that reason.

Mixed pushes follow from the per-file rule: the push deploys when any one of its files would deploy alone. One mixed cell is called out because the per-file reading is itself an inference:

- **C (inferred), row 5, ignored files under the root plus files outside it.** With a Root Directory and ignored paths but no included paths, a push whose files under the Root Directory are all ignored, and whose other files lie outside it, does **not** deploy (`build_filter`). P1 calls a file outside the Root Directory an ignored file, and says a pull request that "only modifies ignored files" makes no preview; R1 and F3 read as two separate tests could instead deploy it. bex takes P1's per-file reading.

The two inferred single-location cells:

- **A (the ambiguous cell), row 6, under the root but not included.** F2 says every non-matching path is ignored once an included path is set, so it does not deploy. P2's "match the root directory or build filter paths" could be read as deploying it. bex takes F2: the sentence is about auto-deploys, P2 about preview environments, and P2 holds anyway for a service whose included paths cover its Root Directory.
- **B, row 6, outside the root and included.** F5 says build filters "can include paths from other directories in your repo"; including a path means F1 applies to it ("Changes that match an included path will trigger an autodeploy"). Render never writes both halves in one sentence, so the cell is marked inferred, but it is what F5 exists to say.

## What changed from bex's earlier rule

Until w7/m428, bex required both tests to pass: some changed file under the Root Directory **and** some changed file passing the build filters (ADR018's Build Filters row called it a "conscious divergence", with no user sign-off). Against that rule:

- **Widened (cell B):** an included path outside the Root Directory now deploys. This is what monorepo shared packages need.
- **Narrowed (cell C):** with no included paths, a push whose only files under the Root Directory are ignored no longer deploys because of an unrelated file outside it. `TestPathsDecisionOnlyWidensButForIgnoredFilesUnderTheRoot` checks over every combination of a small monorepo that this is the only push the old rule deployed and the new one does not.
- Unchanged: every other cell, including A (the old rule did not deploy it either).

## Out of scope

- Skip phrases and filters gate auto-deploys only. A manual deploy, a deploy hook or a settings change deploys whatever the files (M1), and a Blueprint sync is not filtered (B1); bex's Blueprint auto-sync keeps its own path gate.
- Files outside the Root Directory are not made available to the build. Render: they "are not available to the service at build time or at runtime".
- Preview environments are a recorded non-goal.
