# w3 · m636 — Services created before bex's default cutoffs build on Render's dated defaults

**Worker:** worker3 **Goal:** a native service with no version signal builds on the default Render's table gives its creation date, for every date since bex's first service, and no line-pin refresh moves it. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Count the services each band moves on production | 30m | — |
| t002 | Node: Render's 2026-04-21 and 2026-09-17 rows | 30m | t001 |
| t003 | Ruby: Render's 2025-06-12 → 3.4.4 row | 25m | t001 |
| t004 | Python: Render's 2026-02-11 band | 45m | t001 |
| t005 | uv and Erlang/OTP: Render's dated tables | 40m | t001 |
| t006 | Correct ADR018 and ADR060 | 30m | t002, t003, t004, t005 |
| t007 | Verify on dev-3 and production | 45m | t006 |
| t008 | Simplify | 15m | t007 |
| t009 | Test coverage | 30m | t007 |
| t010 | Closeout | 10m | t009 |

## Definition of done

1. For Node (and the companion Node), Ruby, Python, uv and Elixir's Erlang/OTP, a service created on any date from 2026-06-05 resolves to the value Render's version page gives that date; tests cover each boundary.
2. A test that bumps each line's reviewed patch shows no dated service's default moves.
3. ADR018's Runtime row no longer carries the Python and Elixir residuals and says truthfully what a line bump still moves; ADR060's table bolds 3.14.
4. On production a rebuilt pre-cutoff first-party service logs Render's dated default and serves 200.

## Source + Goal linkage

- **Source:** `/pm-brainstorm one render aspect for w1` (2026-11-25), each claim re-checked by an independent verifier agent the same day before filing, item 5. It was proposed as a decision note. Verification found the bands are recorded only as residuals with no user sign-off (w8/174 asked for one; ADR018's Runtime row and ADR060 record them, and the Node and uv bands are not in ADR018 at all), so under AGENTS.md ("Never park parity work on an optional decision. Ship the parity-sufficient choice") this milestone ships Render's behavior.
- **Goal linkage:** [ADR008 § Render parity first](../../../docs/ADR008-vision.md#render-parity-first); the AGENTS.md rule that a divergence stands only when ADR018 records it with the user's sign-off.
- **Expected outcome:** a no-signal native service builds on the version Render gives a service created the same day, and stays there through every line refresh (today these services move whenever a line is bumped: Node 24.21.0 → 24.22.3, Python 3.13.15 → 3.13.18, Ruby 3.4.11 → 3.4.12 since October).
- **Why now:** every refresh moves these services again ([m635](../m635/README.md) is about to move Ruby 3.4.12 → 3.4.13), and [m637](../m637/README.md)'s whole-table comparison would otherwise need exceptions no one signed off.
- **Risk:** services created 2026-02-11 to 2026-10-23 with no Python signal move 3.13 → 3.14 on their next build, and Node/Ruby services move down to Render's older patches (24.14.1, 24.21.0, 3.4.4). That is what Render gives such services. Precedent: w4/620, w9/138 and w8/371 already moved existing services onto Render's dated defaults. Holding Python, or freezing all bands at today's patch, would be a divergence needing the user's sign-off in ADR018; it is not filed.
- **Render parity omitted:** no REST, GraphQL, MCP or dashboard change; build resolution and build-log lines only.
- **Out of scope:** a service of unknown creation time keeps the line's pin.
