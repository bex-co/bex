# Native CLI corroboration of w4/m200

Functional CLI sweep111, 2026-10-11 UTC, Bex v0.3.2 and unmodified Render v2.27.0 at a764810a7682, targeting Bex with isolated human OAuth in bex-canary. Source HEAD 7d23ecb2c29fbab76d86aca3297ce230810f8130; backend/operator/node revisions unobserved.

Both native live tails return five records, but only the short stdout, short stderr and stdout-only long control match their original messages. The expected mixed stdout (48,072 bytes) and stderr (94 bytes) become a joined 32,862-byte record and an orphan 15,304-byte suffix. Exact comparison proves the joined record is the first 32,768 stdout bytes plus the complete stderr message, and the orphan is the remaining stdout bytes. Both clients' history returns all five originals exactly once. Cross-stream ordering is not asserted; counts alone would miss the defect.

This corroborates [existing w4/m200](../../../w4/blocked/m200/README.md) through native CLI tails, whose pinned implementation uses WebSocket. It schedules no duplicate implementation task. The producer remedy, global consumer audit, timestamps and node artifact verification remain with that milestone. Native wire was not intercepted, and no deployed revision is inferred from source or image-pin history.

[Complete native commands/streams, expected messages and trigger](native-live.json), [historical controls](history.json), [successful fixture creation/readiness](fixture.json), [complete program](program.py.txt), and [cleanup](cleanup.json). Every emitted message is synthetic and below 64 KiB; this is distinct from w4/256's over-cap defect.

One owned Free Python web service, srv-db5nnubnt75c73b8g5o0, was created natively, reached Live and returned the exact readiness marker. Its batch endpoint emits at most once per named batch. Both streams were deliberately interrupted after observation (SIGINT -2, no forced kill), reaped, and the temporary Render config copies removed. The isolated QA grant remains active for the requested loop.

Native delete exited 0 in 1.312s; detail/public runtime returned 404, the native list contains only the six immutable baseline IDs, and service termination plus PG/KV usage/termination are zero. No owned fixture remains. No product/node change, commit, release or external submission occurred.
