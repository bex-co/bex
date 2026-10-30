# Bounded full-byte review concurrency — 2026-10-30

The private V9 driver adds bounded parallel reads to the final anonymous config/layer SHA-and-size stage. It reuses the existing validated worker option (default two, range one to four), canonical public auth/pull/cache code and serialized progress. Every descriptor must succeed before final approval; results keep the original order. Failure cancels only unstarted own requests and joins started reads. The earlier decompression phase is already joined, so concurrency is not additive. Stream bytes remain discarded after hashing.

Four local tests exercise the real canonical hash and locked cache code with small mocked stream bodies: overlap, out-of-order completion, preserved cache rows/HEAD reuse, single-worker behavior, the two-GiB floor, and wrong/truncated/overlong rejection before approval while joining a deliberately held started read. The existing 43 strict guards pass on the exact V9 hash. These are local test controls, not public pulls or artifact approval. An initial mocked-HEAD fixture omission is retained alongside the corrected pass.

Root's quality review and independent reuse/efficiency reviews accepted the exact implementation. Source reversal reconstructs V8 byte-for-byte outside the helper, final call and additive receipt metadata. Full source, unchanged policy, test code, logs and hashes are retained losslessly. Final streams enforce the two-GiB reserve; external case/global-space supervision remains required.

No real case has run on V9 here. Ruby 3.1 continues on unchanged V8. A future case still requires all actual signed reports, source/subject controls, both architecture bindings and complete anonymous bytes. The production catalog and all original hosted DoDs remain open.
