# w8 · m79 — Supply automatic Brotli for static sites

**Worker:** worker8 **Goal:** native static deployments serve eligible assets with Render-compatible automatic compression. **Status:** done — all five tasks; deployed acceptance and exact cleanup in sweep340

## Tasks (in order)

| ID | Task | Estimate | Depends on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | [Implement bounded automatic Brotli for static responses](done/t001.md) | 55m | — — **DONE 2026-10-15** |
| [t002](done/t002.md) | [Render parity and shared-path verification](done/t002.md) | 35m | t001 — **DONE 2026-10-15** |
| [t003](done/t003.md) | [Simplify the changed code](done/t003.md) | 20m | t002 — **DONE 2026-10-15** |
| [t004](done/t004.md) | [Test encoding and neighboring HTTP contracts](done/t004.md) | 35m | t002, t003 — **DONE 2026-10-15** |
| [t005](done/t005.md) | Closeout after native hosted acceptance — **DONE 2026-10-16** | 25m | t004 |

Five tasks,170m. [Finding](finding.md) · [native/runtime/source/cleanup evidence](evidence/cli-sweep326/README.md).

## Definition of done

- Fresh owned Free static deployments through Bex and unchanged pinned Render reach Live; br HTTP responses for eligible text/JS/CSS are compressed, smaller and decode to the exact original bytes. Identity/q=0 and authored precompressed assets remain correct.
- Maintained behavior tests cover all three serving callers, HTTP framing/preconditions/ranges/HEAD, Vary/no-transform, shared memory/CPU/cache bounds and publication revision isolation. Relevant checks pass.
- Existing REST/GraphQL/MCP/dashboard configuration surfaces retain common serving semantics, with no new public field or CLI fork. Official parity and all untested paths are recorded honestly.
- Exact-owned fixture/child absence, baseline retention and quota recovery are verified. Complete task statuses and move the entire milestone to done only after observable acceptance, leaving no stub.

## Source + Goal linkage

- **Source:** functional $qa-find-bugs-cli w8/sweep326 on2026-10-15. Two native Free creates and24 correct-content but uncompressed responses; three actual Handler Brotli failures and four controls.
- **Goal linkage:** ADR008 dependable Render-alternative hosting, ADR018 static-site compatibility, ADR029 bounded shared static serving.
- **Expected outcome:** ordinary eligible assets transfer fewer bytes to Brotli-capable visitors, with exact decoded content and correct HTTP/cache semantics.
- **Why now:** both clients produce healthy sites yet the entire serving path omits automatic encoding. This exceeds a sub-hour note because response negotiation must preserve saved headers, ranges/validators and shared budgets.
- **Render parity included:** static hosting is tenant-facing; the target is Render's documented Brotli behavior. Identity is protocol-valid, so this is a minor compatibility/performance gap rather than an HTTP conformance failure.

The original326 receipts establish the historical gap. Implementation5a39b80ff and the deployed acceptance below complete it; QA supplies no product implementation or platform deployment.

## Closed 2026-10-16 — sweep340

[Full two-client hosted acceptance](evidence/cli-sweep340/README.md):24 correct-content responses include12 actual Brotli representations of text/JS/CSS, all smaller and independently decoded. Another28 negotiation/framing/conditional/range/authored-header checks pass. Seventeen maintained serving/startup tests pass. Both Free parents and every recorded deployment child are absent with baseline/quota preserved. The API-only header setup and small precompressed-control scope are explicit. No browser or independent Render-hosting coverage is claimed.
