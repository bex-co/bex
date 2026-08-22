# w5 · m77 — ACP profile and stream hot-path simplification

**Worker:** worker5 **Goal:** Apply the useful design lessons from Vercel's `@ai-sdk/harness-acp` profile model without adopting its sandbox orchestration or AI SDK v7 runtime: keep bex's direct official ACP client, OpenSandbox, credential gateway, transcript authority, and AI SDK v6 wire contract while consolidating reviewed agent profiles and removing per-part filesystem/database work from the live stream path. **Status:** todo

## Tasks (in order)

| id   | title                                                                    | est | depends_on |
| ---- | ------------------------------------------------------------------------ | --- | ---------- |
| t001 | Baseline the current ACP stream path and freeze the adoption boundary    | 45m | —          |
| t002 | Consolidate reviewed ACP runtime profiles into one release-locked manifest | 60m | t001       |
| t003 | Batch transcript persistence without blocking browser forwarding         | 75m | t001       |
| t004 | Buffer driver logging and reuse encoded stream frames                    | 60m | t001       |
| t005 | Simplify — `/simplify` over profile, driver, gateway, and store changes  | 30m | t002, t003, t004 |
| t006 | Test coverage — profile validation, batching, ordering, and failure recovery | 60m | t005       |
| t007 | Closeout                                                                 | 10m | t006       |

## Definition of done

- Production still uses the in-pod direct `@agentclientprotocol/sdk` client, existing OpenSandbox lifecycle, gateway credential brokering, and AI SDK v6 UI-message stream; neither `@ai-sdk/harness` nor `@ai-sdk/harness-acp`, a second bridge, a WebSocket hop, or a second sandbox provider is introduced.
- One release-locked, non-secret profile manifest is the source for supported profile id, executable, args, bootstrap environment, credential environment, and model-proxy base-URL routing; backend and driver validation reject unknown or incomplete profiles, and the image verifies every declared executable.
- The live gateway forwards accepted UI-message parts without waiting for one PostgreSQL transaction per part. Transcript writes are bounded and batched, preserve `(session_id, turn, part_index)` ordering/idempotency, flush at terminal boundaries, and retain ADR051 log harvest as the recovery backstop.
- The driver keeps one bounded append sink per turn instead of opening the transcript log for every part, and its hub reuses one encoded SSE frame for byte accounting, replay, eviction, and fan-out.
- A repeatable benchmark/test fixture records before/after transaction, file-open/write, serialization, ordering, and first-part-forwarding behavior. The optimized path shows fewer persistence operations without changing replay bytes, truncation limits, redaction, error convergence, or dashboard/mobile rendering.
- ADR047/ADR051 record the Vercel compatibility lesson and the retained adoption boundary so a future AI SDK release does not reintroduce the rejected duplicate orchestration.

## Source + Goal linkage

- **Source:** user handoff on 2026-08-22 after reviewing Vercel's “Use ACP-compatible harnesses with the AI SDK harness layer” changelog against the shipped architecture; follows, but does not duplicate, `w5/m66`, which already removed the fake-LanguageModel/`streamText` ACP detour.
- **Goal linkage:** ADR008 pillar 5 and ADR047's cloud coding-agent conversation plane — lowers per-part latency and maintenance cost in the AI-native execution path while preserving bex's platform-owned isolation, credentials, lifecycle, and durable transcript boundaries.
- **Expected outcome:** adding or updating an ACP runtime becomes a reviewed profile-data change rather than synchronized Go/TypeScript conditionals; token-sized stream updates reach the browser independently of PostgreSQL latency; transcript logging and fan-out perform bounded batched work rather than repeated per-part setup.
- **Why now:** the new Vercel meta-adapter validates the profile abstraction but remains an experimental AI SDK v7 sandbox-orchestration layer. bex has already paid for and hardened the equivalent platform mechanisms, while the current gateway demonstrably waits on a transaction/advisory lock/`max(seq)` query before forwarding each part and the driver opens the log for each chunk. Taking the profile and batching lessons now yields measurable simplification without a stack migration.
- **Render parity omitted:** this is an internal agent-session mechanism/performance milestone. It changes no REST, GraphQL, MCP, dashboard, or mobile fields or semantics; consumer consistency and byte identity are explicit test gates instead.
