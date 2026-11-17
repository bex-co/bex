# w22 · m444 — Render CLI v2.37.0: adopt it, with images list and delete and the interactive sandboxes list

**Worker:** worker22 **Goal:** the `bex` launcher pins Render CLI v2.37.0, and the unmodified v2.37.0 binary works against bex for what the release added: `sandbox-groups images list` with its filters, `sandbox-groups images delete` with its preview and in-use refusal, and `sandboxes list` in both its interactive and text output. **Status:** todo

## Tasks (in order)

| id   | title                                                                                                                                     | est | depends_on       |
| ---- | ----------------------------------------------------------------------------------------------------------------------------------------- | --- | ---------------- |
| t001 | Read the v2.36.0 to v2.37.0 client diff: which routes, query parameters and error codes the new and changed commands use                  | 40m | —                |
| t002 | Move the pin: `UPSTREAM_RENDER_CLI.md`, the README's version wording and `lego/cli/testdata`                                              | 30m | t001             |
| t003 | Close the server differences t001 found: path spelling, what the delete preview reads, and the shape of the dependents an in-use error names | 60m | t001             |
| t004 | Branding overlay: rewrite the new help text's render.com links and paths, with tests                                                      | 30m | t002             |
| t005 | `docs/cli-compatibility-checklist.md`: a v2.36.0 to v2.37.0 delta section, every new row graded                                           | 30m | t003             |
| t006 | Production walk: the unmodified v2.37.0 binary through the `bex` launcher runs images list and delete and both outputs of `sandboxes list` | 45m | t004, t005       |
| t007 | Render parity                                                                                                                             | 20m | t003, t004, t005 |
| t008 | Simplify                                                                                                                                  | 20m | t007             |
| t009 | Test coverage                                                                                                                             | 30m | t007             |
| t010 | Closeout                                                                                                                                  | 10m | t006, t008, t009 |

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
