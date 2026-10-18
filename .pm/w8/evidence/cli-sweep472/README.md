# Pre-deploy success/failure legs — sweep472

Passing functional coverage, no new issue; the loop now runs inline (cron deleted at the user's request). Same isolated session as sweeps 454–471, `bex-canary`, HEAD checkout build (`9cc884ebb`), 2026-10-18T07:31–07:36Z.

Pre-deploy both legs on a native Node service: success leg flows `queued → build_in_progress → pre_deploy_in_progress → update_in_progress → live` with the planted `qa472preok` line present in build-type logs; then `--pre-deploy-command 'echo qa472prefail; exit 3'` and a waited deploy terminates `pre_deploy_failed` (~75 s, CLI exit 1) while the prior release keeps serving HTTP 200 and the superseded config-change deploy reads `canceled` — failure isolates to the new release exactly as recorded.

Cleanup: exact-owned `srv-dba7a3monb0s73deeji0` deleted by ID (404-verified); baseline deltas are concurrent foreign churn, observed read-only. No plan mutations. Session remains active; the loop continues inline from sweep473; m116 + evidence 465–472 remain uncommitted pending `/ship` (m115 shipped in 72a8d9ea4 and is being worked upstream).
