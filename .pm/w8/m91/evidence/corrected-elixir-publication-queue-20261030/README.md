# Corrected Elixir publication scheduling — 2026-10-30

The coverage-preserving scanner fix shipped as846a56038319f1b5c212d1fcda8291d20e49c2b0. Its automatic full push run41121784740 received exactly one cancellation after two fresh complete job inventories showed zero represented runtime builds and no assigned/started runtime build. Later actual metadata confirms completed/cancelled: matrix checkout was cancelled, matrix expansion skipped, and a generic cancelled `build` placeholder has no runtime matrix name or executed steps. The source/observations and actual terminal JSON are retained; no running compile was cancelled. The earlier in-progress/queued observations were not terminal proof.

Fresh main/source and manual-run preflight preceded exactly one POST per corrected target. Each returned one unique new actual run. Actual singleton runtime job names then resolved:

| Runtime | Run | Job | Source | Observed state |
| --- | --- | --- | --- | --- |
| Elixir1.18 |41121900239|124293122936|846a56038319f1b5c212d1fcda8291d20e49c2b0| queued |
| Elixir1.19 |41121921308|124294176308|846a56038319f1b5c212d1fcda8291d20e49c2b0| queued |

Only those two watcher targets were replaced. Their old failed source99f95 runs and every prior observation remain retained. Other active and queued suppliers were untouched. No mass replacement, duplicate target POST, artifact verification or supplier approval is implied. Seven suppliers remain approved; corrected complete scans/signing, all27 adoption and original hosted acceptance remain open.
