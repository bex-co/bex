# Wrong local file selection reaches the live service — sweep226

Published Bex and the unmodified Render pin each create a fresh Free native Node service with qa-note.txt pointing to the local path neighbor.txt followed by space. Both deploys reach Live and their own HTTP endpoints return NEIGHBOR_A plus LF, while the literal intended file contains INTENDED_SPACE_B plus LF. A separate Bex native service using the ordinary interior-space path returns INTERIOR_E plus LF exactly. The process reads the mounted file directly; this confirms the two wrong-file cases and one good control at runtime.

Native create/deploy histories and HTTP readbacks are retained. Production POST bodies were not intercepted in this follow-up; the exact unchanged actual-client request bodies remain in the local sweep225 receipts. Deployed API/operator revision is unobserved, and Git source commit is recorded separately in native history. The first image/start-command combination was rejected by the pinned command parser; its name stayed absent and no ID was created. That setup error is preserved and excluded.

All three exact ledger-owned services and their initial deploy children were deleted. Each detail/list/former HTTP endpoint is absent; read-only quota reconciliation reaches six baseline services, zero terminating, PG0/KV0. No baseline mutation or second delete was sent. Temporary Render config copies are gone. QA/hook values are redacted; all file contents are harmless markers.

- [Native creation, process readback and complete cleanup](owned-native-runtime-cleanup.json)
- [Fixture driver](live-fixture-driver.py.txt)
- [Cleanup driver](cleanup-driver.py.txt)
- [Versions, redactions and scope](metadata.json)
