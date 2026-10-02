# w4 · m150 — Apply saved static header wildcard patterns

**Worker:** worker4 **Goal:** a saved static response-header rule matches Render's documented root-file and nested-path wildcard patterns on the served response. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Match documented header patterns in the static server | 50m | — |
| t002 | Audit shared consumers and preserve route and error contracts | 30m | w4/m150/t001 |
| t003 | Render parity across header configuration and serving | 25m | w4/m150/t001, w4/m150/t002 |
| t004 | Simplify the milestone changes | 20m | w4/m150/t003 |
| t005 | Test header selection and preserved response behavior | 40m | w4/m150/t003, w4/m150/t004 |
| t006 | Closeout after the live acceptance passes | 15m | w4/m150/t005 |

Total: **6 tasks, about 3 hours**. This filing ships the researched work, not its implementation.

## Definition of done

Replay the disposable no-build static site recipe and complete requests in [finding.md](finding.md). These targets come from probes actually run in sweep 11; untested adjacent cases are assigned to the tasks.

- Save the eight header rows from the finding through the dashboard, reload, and wait until the updated `X-QA-All` marker appears on the public URL. GET/HEAD `/render.yaml` return the original file with `X-QA-Pattern: r11-root-yaml` and the exact/catch-all controls. GET/HEAD `/missing-r11.yaml` remain 404 and include the same root-pattern header.
- With no catch-all rewrite, GET/HEAD `/nested/missing-r11.yaml` remain 404 and include `X-QA-Nested`, `X-QA-Nested-All`, prefix and catch-all markers. They do **not** receive the root-only `X-QA-Pattern`. `/nested/` remains 200 with the nested-all and prefix markers. Root `/render.yaml` does not receive the nested-only markers.
- Toggle the formerly failing row from `/*.yaml` to exact `/render.yaml` and back, checking after each snapshot refresh. Both configurations produce the marker on the root file. Fresh dashboard, REST GET, GraphQL read and MCP list retain the exact accepted patterns, with no rebuild or revision change.
- Save the three recorded route controls, remove and restore the catch-all rewrite, and reload. Existing `/render.yaml` still serves its file with 200/no Location; `/qa-old` remains 301 to `/index.html`; `/qa-route` receives request-path/catch-all markers and no index-only marker. The nested wildcard headers also reach a rewritten nested request.
- Delete the owned fixture. Its service/header REST reads become 404, list omits it, GraphQL returns null/not found, and exact-identity artifact checks converge to empty. Revoke only that verification session. Complete the explicitly unverified sibling cases and meaningful tests in t002–t005 before closing.

## Source + Goal linkage

- **Source:** continuous `$qa-find-bugs`, production sweep 11 on 2026-10-02, using `muse.env`, filed to w4 by user request. [Finding](finding.md) contains exact requests/responses, two reproductions, the same-rule positive control, caller counts, the complete prior-DoD disposition, and cleanup.
- **Goal linkage:** ADR008 Render-compatible hosting, [ADR029](../../../docs/ADR029-static-sites.md) static serving, [ADR018](../../../docs/ADR018-render-parity.md) header parity, and [ADR006](../../../docs/ADR006-bex-api.md) shared API contracts.
- **Expected outcome:** file-extension and nested-path header policies accepted by the product are actually emitted on matching responses; a customer's cache/security policy is not silently omitted by path selection.
- **Why now:** both API and dashboard report successful persistence while the runtime compares unsupported patterns literally. This survives a fresh page and a refreshed server snapshot, including errors where existing catch-all rules already work.
- **Render parity included:** the served HTTP surface and its existing REST/GraphQL/MCP/UI configuration contract are user-visible. [Render's header pattern table](https://render.com/docs/static-site-headers) is the primary contract; authenticated Render execution remains unverified.
- **Dedupe:** residual grammar gap from w1/m21; different from the passing w4/m94 precedence/request-path and w4/m101 error-header fixes. No open match and no pending-main fix found. Cache purge, individual-rule APIs, route placeholder expansion, external redirect policy and implicit-SPA policy are out of scope.
- **Safety and limits:** the QA fixture and session are already cleaned up. Only harmless marker headers were tested. Production overload, origin failures, other tenants and security-header mutation were not exercised; use isolated fixtures/fakes for those tests.
