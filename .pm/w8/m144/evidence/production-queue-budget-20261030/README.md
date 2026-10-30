# Observed production queue and paid-test budget — 2026-10-30

The platform and new CSI have actual Ready evidence. No paid disk service, create intent, release gate or armed watchdog exists. Before creating the unchanged single Starter/1→5Gi fixture, the current queue and post-create work must credibly fit7200seconds including at least900seconds for cleanup.

[Retained empirical observations](observations.json) establish actual concurrent production jobs on three distinct runner names, not idle availability or an observer/backup reservation. Platform deployment queued30m39s and executed19m37s; CSI queued2h49m27s and executed3m08s. At20:15:20 the independent inventory had waited72m19s since its production job was queued and104m39s since dispatch, without observed execution. Counting that dispatch wait plus900seconds leaves21seconds for work and further waits. These observations do not establish a credible complete acceptance fit. No required two-idle-runner or org-runner visibility gate is added.

The observer's6000seconds is a maximum, not a mandatory minimum; actual observer/backup overlap and shorter work can inform a later current-capacity review. The fixture budget, data and programs are unchanged. No timeout is treated as a reason to increase spend or leave a resource beyond cleanup. This checkpoint records an unexecuted paid test and closes no task.

A separate [owned cache cleanup receipt](completed-owned-cache-cleanup.json) records removal only of the completed CSI Linux-test Go build cache after a no-open-files check. All sources, test binaries and evidence remain. No shared or foreign cache was pruned.
