# CLI sweep 632 — the secret-file cap counts serialized bytes: boundary between 524200 and 524280

**Verdict: no bug filed.** Same isolated QA session, `bex-canary`, CLI at `c8717afdf`, 2026-10-19. Owned fixture `qa632-web` (`srv-dbbaviqvfpec73dm0hmg`, free busybox), deleted at close: REST 404, zero `qa632` names. Contents were filler bytes.

- **Bisection on a single file (empty set each time):** raw contents of 393216, 393700, 394000, 400000, 450000, 500000, 520000, 524000, 524100, and 524200 bytes all stored (`201`). Contents of 524280, 524287, 524288, and 524289 bytes all refused with `400 total secret file size limit of 524288 bytes exceeded`.
- **What the cap measures:** the cut sits ~200–280 bytes below the content size, i.e. the budget is spent on the serialized entry (content plus name and framing), not the raw content alone. That reconciles every earlier observation: two 256KiB group files (sweep 631) exceed it jointly, a bare 524287-byte file exceeds it alone, and mid-size files pass. My mid-sweep base64 theory was wrong and was discarded against this cleaner bisection; a connection reset while reading one large echo response also briefly suggested a lost file — a re-list proved the set intact (files coexist per name; sweeps 629–630).
- **Refusals are atomic:** after every refused PUT the stored set was byte-identical.

## Unrun

- The exact serialized overhead per entry (name-length dependent, presumably).
- Binary (non-UTF8) secret contents through this path.
