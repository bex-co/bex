# Mixed-type log page + KV persistence toggle — sweep481

Passing functional coverage, no new product issue; ship state unchanged (gates re-run: `main` still red at `e6457568b`, run 39120931451; held commit `f62d6740e` remains local, nothing pushed). Same isolated session as sweeps 454–480, `bex-canary`, HEAD checkout build, 2026-10-18.

Mixed-resource log read (`-r <web>,<kv> --limit 6`) returns one globally ordered page across kinds — here saturated by the Key Value instance's newer startup lines (all six entries carry the `red-` resource label), consistent with the single-page global-order design rather than per-resource quotas. Key Value `--persistence-mode` toggles `journal_snapshot → off → journal_snapshot` with exact readbacks (transient `config_restart` on change).

Cleanup: exact-owned `srv-dbaic2pn2qrc73fmlpp0` and `red-dbaic2pn2qrc73fmlpqg` deleted by ID (404 / named not-found verified); baseline deltas are concurrent foreign churn. No plan mutations. Session remains active; next inline sweep is 482; `f62d6740e` + evidence 477–481 await a green main.
