# Actual platform scan transport failure — 2026-11-01

[The full failed-job evidence](RAW-CUSTODY.json) shows run41254794386/source818 passed all five image builds and signing/full SBOMs, then failed at its first Trivy DB download before analysis. The GHCR database transfer reached76.23/131.17MiB before the300-second deadline; deployment was skipped. No qualified rollout follows from these built and signed images.

Peer5f20e81ab6bb82ab125d8d4cb89952c5833936dd already addresses this exact run with bounded per-repository database retries and reuse of the downloaded database. That fix is included in94cf; no successful fixed replay is observed here. The last actual green deploy predatesa154, so it does not satisfy the compression source gate. No duplicate patch, rerun, cancellation, inventory, login, fixture or rollout was performed by this observation.
