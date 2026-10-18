# Postgres external allowlist enforcement — sweep482

Passing functional coverage, no new product issue; ship state unchanged (gates re-run: `main` still red at `e6457568b`, same failed run; held commit `f62d6740e` remains local). Same isolated session as sweeps 454–481, `tian-personal` for this fixture, HEAD checkout build, 2026-10-18.

External-access semantics proven end to end from the runner (bounded `bex psql` probes): default create (both-family open entries) connects; restricting to a documentation `/32` makes the launcher refuse before dialing ("IP address (…) not in allow list for …", exit 1, no query sent); `--clear-ip-allow-list` reads back `[]` and means **fully closed**, not reopened (still refused after a 50-second lag check); IPv4-only `0.0.0.0/0` passes the preflight but the proxy drops the runner's IPv6 connection (SSL error), and restoring both `0.0.0.0/0` + `::/0` connects again. Per-family exactness throughout — no finding.

Cleanup: exact-owned `dpg-dbaid5pn2qrc73fmlq90` deleted by ID (named not-found verified); canary baseline deltas are concurrent foreign churn. No plan mutations. Session remains active; next inline sweep is 483; `f62d6740e` + evidence 477–482 await a green main.
