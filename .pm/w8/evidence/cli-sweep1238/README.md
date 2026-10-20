# CLI sweep 1238 — first-pull timing for uncached prebuilt images (live, 2026-10-20)

Four free web services created with `bex services create --image … --plan free`, each deleted by ID (GET 404). The ledger audit shows no surviving fixture. No new bug filed.

| Image (first use in this loop) | Instance created → first app log | Create → live |
| --- | --- | --- |
| `ghcr.io/stefanprodan/podinfo:6.7.0` | 5 s | never — listens on 9898 and ignores `PORT`. The stall reason correctly names the TCP check on port 3000. Web-service port detection is the documented refusal (w2/m95 option (a); w6/m148 out of scope), so it was not filed. |
| `docker.io/ealen/echo-server:0.8.14` | 78 s | 119 s |
| `us-docker.pkg.dev/google-samples/containers/gke/hello-app:2.0` | 3m44s | 245 s |
| `docker.io/ealen/echo-server:0.7.0` | 33 s | 59 s |

Together with sweep 1237 (`gcr.io/google-samples/hello-app:2.0`, ~7 min) and sweep 1233 (a native Node image from the in-cluster registry, ~13 min, not reproduced in 1235), first pulls range from seconds to many minutes. No `stallReason` explains them while they run. Attribution needs the pod's pull events, which QA authority cannot see, so this stays an observation. If a repeatable in-cluster (Zot) pull stall appears, file the missing pull/scheduling diagnosis (`stallDiagnosis`, `lego/backend/internal/store/reconciler.go:1832–1838`) with that evidence.
