# w8 · m86 — Install automatic npm dependencies for static builds

**Worker:** worker8 **Goal:** A declared npm static project builds with its normal authored script, without a manual installer prefix. **Status:** done — 2026-10-16; source tasks and fresh hosted default/manual acceptance pass; all owned fixtures cleaned

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [Install automatic npm dependencies for native static builds](done/t001.md) | 45m | — — **DONE 2026-10-16** |
| t002 | [Audit shared roots, environment and install selection](done/t002.md) | 25m | t001 — **DONE 2026-10-16** |
| t003 | [Verify Render parity across static authoring surfaces](done/t003.md) | 20m | t002 — **DONE 2026-10-16** |
| t004 | [Simplify the changed installation path](done/t004.md) | 10m | t003 — **DONE 2026-10-16** |
| t005 | [Test actual dependency use and phase controls](done/t005.md) | 35m | t003 — **DONE 2026-10-16** |
| t006 | [Verify live default installation and close the milestone](done/t006.md) | 25m | t004, t005 — **DONE 2026-10-16** |

**Total:**6 tasks/160m. [Researched finding](finding.md) · [full evidence](evidence/cli-sweep358/README.md).

## Definition of done

Fresh original Free npm static deployments through published Bex and unchanged pinned Render reach Live/HTTP200 at exact source/commit, with no authored install prefix and SKIP_INSTALL_DEPS omitted. The normal build script uses its declared locked dependencies and a separate HTTP read matches the generated-page hash. Explicit true/manual install, false/default, nested/no-manifest, effective env/cache and existing nonstatic publication controls pass. Shared source adapters preserve command/env semantics. Actual artifact tests and required suites pass. Exact own fixtures/children/URLs are deleted with original baseline/quota intact. Other manager/version/UI/runtime limits are stated. Completion includes the standing tasks and actual done-folder moves, not merely source implementation.

## Source + Goal linkage

- **Source:** Authorized CLI functional QA sweep358,2026-10-16; [two original build failures and two installed controls](finding.md).
- **Goal linkage:** [ADR008](../../../../docs/ADR008-vision.md), [ADR018](../../../../docs/ADR018-render-parity.md) and [static hosting ADR029](../../../../docs/ADR029-static-sites.md): existing customer build commands must work against the supported static contract.
- **Expected outcome:** An unambiguous locked npm static project automatically obtains its dependencies before its authored build; explicit skip retains manual control.
- **Why now:** Both native clients fail the first build of a real public Eleventy project because dependencies are absent; the one-line installer controls isolate the gap. Existing pnpm-tool and static-compression milestones did not certify automatic project installation.
- **Render parity:** Included because this changes tenant-facing static build behavior across existing authoring surfaces. No new wire field, CLI fork or product toggle. Implementation10dda1f1c; hosted closeout in sweep379. No API/CLI fork or new product toggle.

## Hosted closeout — sweep379

[Fresh native default and skip/manual acceptance](../../evidence/static-auto-deps-sweep379/README.md) passes all three first deployments, exact source/dependency/generated-page proofs, nine shared readbacks and complete cleanup. Required source execution/neighbor checks remain those recorded in t001–t005. t006 is done; all tasks and this milestone move to done. Browser, exact serving revision and hosted other-line/root/manager variants are not claimed.
