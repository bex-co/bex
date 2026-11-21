# w22 · m444 — Render CLI v2.37.0: adopt it, with images list and delete and the interactive sandboxes list

**Worker:** worker22 **Goal:** the `bex` launcher pins Render CLI v2.37.0, and the unmodified v2.37.0 binary works against bex for what the release added: `sandbox-groups images list` with its filters, `sandbox-groups images delete` with its preview and in-use refusal, and `sandboxes list` in both its interactive and text output. **Status:** done

## Tasks (in order)

| id   | title                                                                                                                                     | est | depends_on       |
| ---- | ----------------------------------------------------------------------------------------------------------------------------------------- | --- | ---------------- |
| t001 | Read the v2.36.0 to v2.37.0 client diff: which routes, query parameters and error codes the new and changed commands use — **DONE**                  | 40m | —                |
| t002 | Move the pin: `UPSTREAM_RENDER_CLI.md`, the README's version wording and `lego/cli/testdata` — **DONE**                                              | 30m | t001             |
| t003 | Close the server differences t001 found: path spelling, what the delete preview reads, and the shape of the dependents an in-use error names — **DONE** | 60m | t001             |
| t004 | Branding overlay: rewrite the new help text's render.com links and paths, with tests — **DONE**                                                      | 30m | t002             |
| t005 | `docs/cli-compatibility-checklist.md`: a v2.36.0 to v2.37.0 delta section, every new row graded — **DONE**                                           | 30m | t003             |
| t006 | Production walk: the unmodified v2.37.0 binary through the `bex` launcher runs images list and delete and both outputs of `sandboxes list` — **DONE** | 45m | t004, t005       |
| t007 | Render parity — **DONE**                                                                                                                             | 20m | t003, t004, t005 |
| t008 | Simplify — **DONE**                                                                                                                                  | 20m | t007             |
| t009 | Test coverage — **DONE**                                                                                                                             | 30m | t007             |
| t010 | Closeout — **DONE**                                                                                                                                  | 10m | t006, t008, t009 |

## Definition of done

- `bex -v` prints `compatible with Render CLI v2.37.0`, and `lego/cli/UPSTREAM_RENDER_CLI.md` names v2.37.0 with its commit.
- With the unmodified v2.37.0 binary on production, signed in as the QA user:
  - `bex sandbox-groups images list` lists a group's images, and its name and status filters narrow the list;
  - `bex sandbox-groups images delete` previews without the confirmation flag and deletes with it; an image a sandbox still uses is refused, and the error names that sandbox;
  - `bex sandboxes list` opens its interactive list in a terminal, and its text output flag prints the table.
- Every row of the checklist's v2.36.0 to v2.37.0 section is `[x]`, or `[-]` with a named refusal and the reason.
- `cd lego/cli && go test ./...` and the backend suite are green.

## Source + Goal linkage

- **Source:** `/pm-brainstorm one aspect of render parity for w22` (2026-11-16), item 3, filed by `/pmx 1,3,4,5 to w22`.
  - **Render** ([github.com/render-oss/cli/releases](https://github.com/render-oss/cli/releases), read 2026-11-16; the page dates v2.37.0 "17 Nov"). Added: "Sandbox group images can be listed, with filters for name and status"; "Sandbox group images can be deleted. Deletion previews by default and requires a confirmation flag. Images in use by sandboxes or snapshots can't be deleted, and the error names those resources"; `workflows dev` and local flow listing. Changed: "`sandboxes list` opens an interactive list in a terminal. The previous table output is available via a text output flag"; Go 1.27.4. Fixed: "Service commands work with API tokens that lack read access to the service's project or environment".
  - **bex today:** the pin is v2.36.0 (`lego/cli/UPSTREAM_RENDER_CLI.md` line 5, w3/m303). The server already has most of it: the image list takes `name` and `status` (`lego/backend/internal/sandbox/images_surfaces.go` ~line 53), delete exists (w3/m367) and `image_in_use` names dependents (`lego/backend/internal/sandbox/images.go` ~lines 59 and 761). bex serves these under `/v1/sandbox-groups/{id}/builds`. **Not verified:** whether v2.37.0's image commands call `/builds` or an `/images` path; t001 reads the client to find out.
  - No board item or document mentions v2.37 (`grep -rIl 'v2\.37' .pm docs lego/cli` is empty at `63e5097c8`).
- **Goal linkage:** ADR008 § Render parity first; `.pm/DO_NOT_DO.md` (no first-party CLI: Render's official CLI is the client to verify); ADR104 gaps 10 and 15.
- **Expected outcome:** a user who installs the current `bex` gets the command surface of Render's current CLI.
- **Why now:** the pin is one release behind, so the diff is at its smallest. The workflows changes do not concern bex (Render Workflows are a recorded omission) and can be graded in one line.
- **Related items, not changed by this milestone:** `.pm/w3/blocked/m303` still owes its t005 production walk with v2.36.0. This milestone's t006 walks the same sandbox commands with v2.37.0; t006 records that in m303's terms so `/unblock w3` can decide whether it covers t005. `.pm/w8/m442` (contract round 8) changes sandbox build naming in the same package; t003 reads its state before editing `images.go`.
- **Render parity included:** the CLI is a customer-facing surface over REST, so t007 compares the served routes and errors with what the v2.37.0 client expects.

## Outcome (2026-11-21)

The pin is Render CLI v2.37.0 (`11ef59f330dedec290f8d71d89b8f7379bbbed09`). bex-api needed no change: t001 found that every request the new and changed commands send already had its answer, so t003 closed with fixture tests only.

- **t001, client diff (17 commits).** The image commands still call `/v1/sandbox-groups/{id}/builds[/{buildId}]`. `images list` sends `limit=100`, an exact `name` and one comma-separated `status`. `images delete` reads the image first; that read is the whole preview. The client parses no dependents field: it prints the error's status, `code` and `message`. `sandboxes list` pages with `limit=20` in a terminal and takes `-o text` for the table. Upstream moved to Go 1.27.4, and `lego/cli/go.mod` and `lego/go.work` followed.
- **One transport change the release notes do not mention.** `sandboxes exec` no longer sends the command when it mints a run token (upstream `ef214e7`). bex-api has served that form since w3/065. `lego/cli/testdata/sandbox-exec-contract.json` now records the empty mint body.
- **t004, branding.** No new overlay rule was needed. `TestBexNestedHelp` pins the four new or changed help pages. The interactive list shows the workspace name and "Sandboxes", and no product name.
- **t007, Render parity.** No route was added, and REST, GraphQL and MCP share one Service verb for the list and the delete. One difference is filed as `w22/025`: Render also refuses to delete an image a snapshot uses, and a bex snapshot does not hold its source image.
- **t008, Simplify.** The change is tests, fixtures, the pin and documents, with no production code, so there was nothing for `/simplify` to run on. The new CLI tests reuse `runBexAgainst` and the contract loader's shape; the backend tests reuse `newImageFixture` and `sameShape`.
- **t009, mutation spot-checks.** Each was applied, seen to fail, and reverted.
  - The REST list ignoring `name`: `TestImageListFiltersByExactNameAndStatus` fails.
  - `image_in_use` naming no dependents: `TestImageRoutesEmitTheCLIContractGolden`, `TestDeleteImageInUseNamesAtMostTenAndStaysInItsWorkspace` and `TestDeleteImageAsOnRender` fail.
  - The build script's version set back to 2.36.0: `scripts/bex-cli-validate.sh` fails with "build script must embed upstream version 2.37.0".
- **Not done here.** The service-command fix for narrowly scoped tokens was graded from the client diff and the two routes it reads, not walked: the commands that reach it act on a service. Releasing a `bex-cli/v*` tag is `/release`.
- **Found on the way:** Render CLI v2.38.0 was published on 2026-11-20 UTC, during this work. This milestone kept its filed scope; the next bump is `w22/026`.

## Production walk (t006, 2026-11-21)

Production at pin `f797f525f5df`. The launcher was built from this tree at the new pin (`bex -v`: `compatible with Render CLI v2.37.0`), with an isolated config directory. Device login as the QA user through `scripts/qa-login.sh`, workspace `bex-canary`. Fixtures: group `sbg-dc0j0ht2b0us73dobfhg` (`m444-walk`), image `sbb-dc0j0i52b0us73dobfkg` (`alpine:3.20`), sandbox `sbx-dc0j0ld26tvc739p17lg`.

| Command | Exit | Observed |
| --- | --- | --- |
| `sandbox-groups images import alpine:3.20 --name alpine --group <g> --wait` | 0 | `succeeded` |
| `sandbox-groups images list --group <g>` | 0 | the table, one row |
| `… list --name alpine -o json`, `… list --status succeeded` | 0 | the image |
| `… list --name alp`, `… list --status failed --status pending` | 0 | an empty table; `[]` with `-o json` |
| `… list --status done` | 1 | `invalid status "done": use pending, running, succeeded, failed` |
| `sandbox-groups images list` (no `--group`) | 0 | the default group's images |
| `sandbox-groups images delete <image> --group <g>` | 0 | "This command would delete this image:", the image, "Re-run with --confirm to proceed"; still listed afterwards |
| `sandboxes create --group <g> --image <image> --wait` | 0 | `running` |
| `sandbox-groups images delete <image> --group <g> --confirm` | 1 | `received response code 409 (image_in_use): sandboxes that have not terminated still depend on this build: sbx-dc0j0ld26tvc739p17lg; terminate them before deleting it`; the image still read `succeeded` |
| `sandboxes exec <sandbox> -- sh -c 'echo out; echo err >&2; cat /etc/alpine-release; exit 7'` | 7 | `out`, `err`, `3.20.10` |
| `sandboxes list --group <g>` in a pseudo-terminal | quit with Ctrl-C | the interactive list: "bex-canary > Sandboxes" and one row with status, plan, region and age |
| `sandboxes list --group <g> -o text` | 0 | the table, one `running` row |
| `sandboxes list --group <g> -o json` | 0 | the array |
| `sandboxes stop <sandbox> --confirm`, then `… images delete <image> --group <g> --confirm` | 0 | "Deleted this image:"; `images list -o json` is `[]` |
| the same delete again | 1 | `received response code 404 (build_not_found): sandbox image not found in this sandbox group` (the client reads before deleting; the API answers 204 again) |

Nothing is left behind: `sandbox-groups delete <g> --confirm` answered `"deleted": true`, the group list shows only `Default`, the CLI logged out, and both Kratos sessions this run opened were revoked.

**For w3/m303:** that milestone is already closed (its t005 walked v2.36.0 on 2026-11-17, `.pm/w3/done/m303`). This walk repeated, at v2.37.0, its `sandbox-groups create`, `sandbox-groups delete`, `sandboxes create --wait`, `sandboxes exec`, `sandboxes list` and `sandboxes stop`.
