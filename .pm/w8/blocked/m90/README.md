# w8 · m90 — Install Yarn dependencies before static builds

**Worker:** worker8 **Goal:** A supported locked Yarn static project builds with its normal dependency-using command. **Status:** blocked — t001–t005 done; t006 needs the operator published and the hosted both-client fresh static acceptance

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [Install Yarn Classic project dependencies before static builds](done/t001.md) | 40m | — — **DONE 2026-10-17** |
| t002 | [Audit manager selection, root, environment and shared callers](done/t002.md) | 25m | t001 — **DONE 2026-10-17** |
| t003 | [Verify Render parity across static authoring surfaces](done/t003.md) | 15m | t002 — **DONE 2026-10-17** |
| t004 | [Simplify the changed installation path](done/t004.md) | 10m | t003 — **DONE 2026-10-17** |
| t005 | [Test real dependency use, skip and manager neighbors](done/t005.md) | 30m | t003 — **DONE 2026-10-17** |
| t006 | [Accept fresh default deployments and close the milestone](t006.md) | 20m | t004, t005 |

**Total:**6 tasks/140m. [Finding](finding.md) · [complete evidence](evidence/cli-sweep401/README.md).

## Definition of done

After publication, fresh first default static builds through both published Bex and unchanged pinned Render become Live and serve exact generated index/dependency/source/lock proofs without an installer prefix. Actual dependencies are present before the authored command; skip=true/manual and meaningful manager/root/env/PATH/cache/npm neighbors pass. Shared authoring surfaces preserve input and output semantics, required implementation checks pass, and exact own fixtures/children/URLs/local resources are cleaned with original baselines/quota intact. Unsupported manager/version cases are explicitly bounded. Code alone does not close this milestone.

## Source + Goal linkage

- **Source:** Authorized functional CLI sweep401,2026-10-17: both native first builds fail on missing declared Express while Yarn is available; both one-line install controls reach Live and six exact HTTPS200 checks pass.
- **Goal linkage:** [ADR008](../../../../docs/ADR008-vision.md), [ADR018](../../../../docs/ADR018-render-parity.md) and [static hosting ADR029](../../../../docs/ADR029-static-sites.md): supported static customer build commands must work against the existing Render-compatible hosting contract.
- **Expected outcome:** An unambiguous supported Yarn Classic static project obtains its dependencies automatically, with existing skip/manual control and manager boundaries preserved.
- **Why now:** The npm correction deliberately left this manager unverified; fresh independent clients now prove the gap and isolate its source to the shared install step.
- **Render parity:** Included because a shared operator change affects tenant-facing static builds across REST/GraphQL/MCP/Blueprint/dashboard and both CLI clients. No new wire field, CLI fork or product toggle.
