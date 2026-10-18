# CLI sweep 487 — datastore log-type dialect, previews refusal, red-main flake diagnosis

**Verdict: no CLI bug filed. The deploy-gate blocker was a Docker Hub TLS flake in the operator suite; rerun requested.** One free web service and one free Key Value were created, probed, and deleted by ID with absence proofs; the baseline was restored exactly (modulo the concurrent hunt's own churn).

## The CI blocker, diagnosed

`bash scripts/ship-gates.sh origin/main` had been refusing the pending board ship because the latest deploy run for `e6457568b` ended `failure`. Inspecting the failed job's logs showed `199 Passed | 0 Failed` envtest specs, with every failure in `TestValkeyPersistenceEngine` caused by image pulls dying on `Get "https://registry-1.docker.io/v2/": net/http: TLS handshake timeout`. That is infrastructure noise, not a code break from the log-shipper pin commit. A targeted rerun of the failed job was requested (`gh run rerun 39120931451 --failed`, exit 0); at write time the run was `in_progress` after multiple polls — gate recheck happens when it concludes.

## Journeys (all expected outcomes)

| probe | observed result | assessment |
| --- | --- | --- |
| `services update --help` (instance scaling surface) | no instance flag on update | Creating with multiple instances requires a paid plan on Render; the free-path absence is consistent drill. |
| `logs -r <kv> --type request` / `--type build` | precise HTTP `400` (datastore logs support text/time/direction/limit/instance filters) | Correct managed-datastore dialect, same contract verified for app logs in earlier sweeps. |
| `services update <web> --previews automatic` | exit `1`, HTTP `400` "`services previews is not supported by this platform`" | Honest refusal, same as earlier sweeps; the fixtures never gained a previews setting. |

Cleanup: the web service's detail read returned HTTP `404`, the Key Value get failed with the standard not-found text at exit `1`, and the baseline diff showed no missing rows (one new row belonged to the concurrent hunt).

## Safety and unrun checks

- No plan mutation/product code/deletion of foreign resource; the rerun touched only the failed flaky job.
- Secrets unrecorded; login stayed in `/tmp/qa454/cli.yaml` and Kratos files untouched.
- Deploy lag means deployed revisions can precede local `main`; the operator commit pinned a newer bex-alloy image whose revisions may not be live.
