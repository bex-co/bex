# New-source utility qualification progress — 2026-11-01

[Run41273908642](https://github.com/bex-co/bex/actions/runs/41273908642), exact source94cf04f95/main/attempt1, passes its actual plan job124752086203. [The complete plan record](plan-success/RESULT.json.gz) measures checkout at263 seconds (4m23), compared with861 seconds (14m21) in the earlier1307 plan. This is one actual timing observation.

[Observed topology](qualification-jobs/RESULT.json.gz) records the publisher skipped and both architecture jobs materialized. [The04:56 snapshot](amd64-setup/REVIEW.json.gz) finds amd64job124753782310 completed checkout in274 seconds and running setup-go; arm64job124753782246 remains queued under the existing serial matrix. Neither architecture has passed the actual generated utility gate in these retained observations. No baseline, production adoption or hosted acceptance is claimed; candidate source/utilities remain null.

[Raw custody](RAW-CUSTODY.json) retains full actual metadata and source identities without mixing the older1307 run. Both successful complete current-source architecture artifacts remain required before family reuse.
