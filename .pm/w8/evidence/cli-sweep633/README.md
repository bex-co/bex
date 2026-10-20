# CLI sweep 633 — secret file content is a byte-exact text string (control chars, Latin-1 range, emoji)

**Verdict: no bug filed.** Same isolated QA session, `bex-canary`, CLI at `c8717afdf`, 2026-10-19. Owned fixture `qa633-web` (`srv-dbbb295fsg2s73ako6bg`, free busybox publishing `N=<mounted bytes> H=<sha256 prefix>`), deleted at close: REST 404, zero `qa633` names. Completes sweep 632's Unrun item.

- **Payload:** one secret file whose content was every codepoint U+0000–U+00FF plus an emoji (U+1F600), sent as a JSON string.
- **API round trip is exact:** the per-file GET returned a string identical to what was sent (control characters as `\uXXXX` escapes), no lossy normalization.
- **Mount is exact:** the running process saw 388 bytes whose sha256 prefix (`5ae4e27a13097504`) matches the locally computed UTF-8 encoding of the same string. So the contract is "text string, UTF-8 on disk" — arbitrary Unicode survives intact; truly binary payloads must be encoded by the user (base64 etc.), as with Render.

## Unrun

- Invalid UTF-8 smuggled via JSON `\ud800` lone surrogates (Go's decoder stance) — a parser edge, not probed.
