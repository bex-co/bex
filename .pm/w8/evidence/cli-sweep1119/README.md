# CLI sweep 1119 — path-injection ids resolve to not-found (live, 2026-10-20)

One disposable free web service created and deleted (204). No new bug filed.

- Semicolon-suffixed and percent-encoded-slash resource ids → 404: the router treats the whole segment as the identifier, so injected separators never reach a different handler or broaden the match.
