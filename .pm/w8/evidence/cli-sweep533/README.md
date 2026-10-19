# CLI sweep 533 — w8/128 live acceptance after the image pin flipped

**Verdict: w8/128 verified live; no new bug.** Same isolated session, `bex-canary`, 2026-10-19T10:19Z. The deployed platform image is now pinned to `8e57d4eaa121` (the w8/128 fix commit itself, `b27e0b27f`), so this sweep re-ran 128's exact acceptance probe on a fresh owned fixture (create → suspend → restart/deploy refusals → resume → delete):

```text
bex restart srv-dbaut5e05jqs73cjk2s0 --confirm
Error: … 409: conflict: service "srv-dbaut5e05jqs73cjk2s0" is suspended

bex deploys create srv-dbaut5e05jqs73cjk2s0 --confirm
Error: … 409: conflict: service "srv-dbaut5e05jqs73cjk2s0" is suspended
```

Both verbs now name the public `srv-…` ID the caller sent — the `tea-<workspace>-<slug>` internal name from sweeps 503/526 is gone. 128's live acceptance is met; its remaining acceptance items (a Rollback refusal through the shared path, the regression test) belong to the fixing worker's own verification, already shipped.

Concurrent-tree note: this checkout currently holds another worker's in-progress rebase of the w8/129 fix (`ecdd8f690`, conflicted in `docs/ADR018-render-parity.md`). This loop touched none of it — CLI probes and untracked evidence only — and 129's live recheck waits until that work lands and deploys.

Cleanup: exact-owned `srv-dbaut5e05jqs73cjk2s0` resumed and deleted by ID.
