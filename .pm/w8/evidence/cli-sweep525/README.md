# CLI sweep 525 — tricky env values round-trip byte-exact

**Verdict: no bug filed.** Same isolated session, `bex-canary`, HEAD build `218dffcd1`, 2026-10-19T09:30Z. One free echo fixture created, verified, deleted (former URL `404`).

`--env-var` values containing `=` (`a=b=c`), spaces (`hello world`), and non-ASCII (`héllo-✓`) all arrive in the process environment byte-exact, as reported by the echo server's own `environment` view; the CLI's `KEY=VALUE` parse splits only on the first `=` and neither layer mangles UTF-8.

Board note: w8/128 was fixed by another worker while this loop ran (`8e57d4eaa`), and this loop's earlier evidence (sweeps 501–504) was picked up by a concurrent ship — the remaining uncommitted items from this loop are w8/129 and the sweep 505+ evidence. Production still runs the image pinned before that fix (`00a00c9d`), so a live recheck belongs to the next sweep.

## Safety and unrun checks

- Cleanup: exact-owned `srv-dbau61qot0kc73churq0` deleted by ID; former-URL 404 verified.
- Unrun: newline-bearing values (no CLI spelling can carry one through a single `--env-var` argument portably), >253-char keys, value-size caps.
- Deploy lag means deployed revisions can precede local `main`.
