# w8 · m106 — Supply Hugo for the documented static quickstart

**Worker:** worker8 **Goal:** the documented Hugo command builds and publishes without an authored Hugo installer. **Status:** blocked — t001–t005 done; t006 needs the operator published and the hosted both-client unchanged quickstart acceptance

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [Supply pinned compatible Hugo to native static builds](done/t001.md) | 55m | — — **DONE 2026-10-17** |
| t002 | [Audit shared static callers source roots and cache behavior](done/t002.md) | 30m | t001 — **DONE 2026-10-17** |
| t003 | [Verify the documented static Hugo contract across consumers](done/t003.md) | 20m | t002 — **DONE 2026-10-17** |
| t004 | [Simplify the static Hugo supplier change](done/t004.md) | 15m | t003 — **DONE 2026-10-17** |
| t005 | [Test actual generated Hugo builds and publication neighbors](done/t005.md) | 55m | t003 — **DONE 2026-10-17** |
| t006 | [Accept original native Hugo quickstarts and close the milestone](t006.md) | 20m | t004, t005 |

**Total:**6 tasks/195m. [Finding](finding.md) · [complete evidence](evidence/cli-sweep431/README.md).

## Definition of done

The unchanged documented Hugo command builds/publishes the actual committed parent and pinned theme on fresh Free native static fixtures from both clients without an installer prefix. Both first children Live; exact source/index/CSS/proof hashes and shared settings/env match. Meaningful generated-image/static tests fail without Hugo supply, cover reviewed binary/base/architecture and source/root/PATH/env/cache/extraction neighbors, and preserve other tools/managers, static no-CMD and nonstatic/Dockerfile/buildless behavior. The established scope stays static build availability. Exact cleanup preserves baselines/current inventories/quotas and zero termination and removes own material. Diagnostic Debian0.111.3 is not an adopted platform pin; unexercised versions/platforms remain explicit. Code alone does not close the milestone.

## Source + Goal linkage

- **Source:** Authorized ordinary functional CLI sweep431,2026-10-17: both original Free static builds fail missing Hugo; install-only controls generate the real committed sample/theme index and stylesheet. Local real-project controls and actual build exports establish the shared supplier boundary.
- **Goal linkage:** [ADR008](../../../../docs/ADR008-vision.md), [static hosting ADR029](../../../../docs/ADR029-static-sites.md), [native build ADR060](../../../../docs/ADR060-build-worker-reliability-and-performance.md), [Render parity ADR018](../../../../docs/ADR018-render-parity.md): a documented ordinary static generator must be available for its normal command.
- **Expected outcome:** Hugo static projects build through the normal native CLI/static pipeline without owner-supplied platform tooling.
- **Why now:** Two matching original failures and install-only real output controls establish a distinct gap; no Hugo supplier repair was found. Coordinate the common static generator with [m105](../m105/README.md), retaining its separate Ruby/Gemfile scope.
- **Render parity:** Included because the shared static pipeline reaches REST/GraphQL/MCP/Blueprint/dashboard/native consumers. This adds build tooling, not a new runtime/API or a compute-runtime availability promise.
