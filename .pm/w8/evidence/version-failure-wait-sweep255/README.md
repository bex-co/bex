# Failed-version deploy waits and recovery — sweep255

Source `6255ed8b2ca67c8c0f85fd0eadc19bedbbe62459`, Bex production, isolated human-QA workspace bex-canary. Published Bex v0.3.2 and unmodified Render v2.27.0 / a764810a768202704e7206eb7b87a47211fcd98e each use a fresh equivalent Free native Node fixture. Deployed revision is unknown. All controls pass; no new bug was found.

[Complete owned commands, native stdout/stderr, API setup, deploy correlation, runtime/instance observations and cleanup](owned-native-runtime-cleanup.json), [driver](native-driver.py.txt), [case metadata and versions](metadata.json), [redactions](redactions.json).

Both fixtures use public bex-co/bex main/examples/runtime-version/node with its unchanged .nvmrc22. A generated standard-library HTTP program independently reports the build/runtime Node version, original file bytes and harmless MESSAGE marker. Each first release becomes Live and serves exact HTTP200 snapshots on Node v22.23.3.

Because this pin has no environment-update command, setup uses the documented Bex PATCH /v1/services/id/environment extension with a sparse NODE_VERSION value and saveMode save_only. Each response is HTTP200/rolledOut:false, readback has the intended18 or24, and deploy ID sets stay unchanged. Marker values and other keys are preserved. These writes are API-only setup, not native env-edit coverage.

First stage unsupported NODE_VERSION18 and run the actual deploys create id --wait --confirm -o json. The pinned help explicitly promises a nonzero exit for a failed deploy. Each native process exits1 and prints valid JSON for exactly its one newly created build_failed deploy; native history agrees on the same ID/status. Phase-scoped native build logs name NODE_VERSION18 and the supported lines22,24,26. The old public response stays200 with the same decoded Node22 snapshot, and advertised instance IDs remain identical to their baseline. No OCI identity or continuous-uptime assertion is made.

Then stage valid NODE_VERSION24, again without an implicit deploy, and repeat the native waited deploy. Each process exits0 for its exact new Live target. Native build narration names the24 env selector, both build/runtime snapshots report v24.21.0 while .nvmrc remains22 plus newline, the own marker stays exact and returned instance IDs change.

| Client | Unsupported18 wait         | Valid24 recovery wait |
| ------ | -------------------------- | --------------------- |
| bex    | 1 / build_failed / 26.934s | 0 / live / 89.822s    |
| render | 1 / build_failed / 42.82s  | 0 / live / 84.772s    |

All four native waits finish within the private240-second QA bound. Their complete progress stderr and JSON stdout are retained; failed waits are not mistaken for transport errors, older Live rows or wrapper timeouts. Native HTTP traffic was not intercepted. Build-log reads are bounded to100 rows and scoped from the corresponding waited intent. This extends existing m51/version and deploy-wait acceptance; it does not re-certify all runtimes, cancellation/rollback, dashboard editing, every failure category or underlying OCI identities.

Both exact owned services are deleted after enumerating six deploy children. Detail and former runtime return404, native lists omit both IDs and retain all six baseline services; final service quota is used6/terminating0 and managed PG/KV remain unused. Direct live Pod teardown was not inspected. Render comparison configs are removed after each process. No product code, commit or push was produced.
