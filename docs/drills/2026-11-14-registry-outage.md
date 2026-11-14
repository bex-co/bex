# Registry outage drill — 2026-11-14

Milestone `w22/m401`. Question: do `ZotRegistryNotReady` and `TenantRegistryImagePullFailing` fit what a real kubelet and a real kube-state-metrics report when Zot is gone, and do they fire from the repository's own rule file, not only from promtool fixtures?

**Scope: local only.** Nothing was scaled or read on production.

## Environment

| Item | Value |
| --- | --- |
| Cluster | a throwaway single-node kind v0.33.0 cluster (Kubernetes v1.37.0, containerd 2.3.4), deleted afterwards |
| Not used | `scripts/mock-cluster.sh`: it needs `kind` and `clusterctl` (the machine had neither) and a CAPI management cluster, Calico, Traefik and MinIO on an 8 GiB Docker VM. The drill reproduces only what the two alerts depend on |
| Zot | `StatefulSet/zot` in `bex-registry`, image `v2.1.22` (the tag `deploy/gitops/base/zot.yaml` pins), NodePort 30500, a 1 GiB PVC, **no auth** (production has htpasswd and ACLs; irrelevant to the matchers) |
| Pull path | containerd `hosts.toml` for `zot.bex-registry.svc:5000` pointing at `http://127.0.0.1:30500`, as `infra/clusterapi/overlays/hetzner-caph/sandbox-pool.yaml` writes it |
| Tenant pods | namespace `tea-drill`: `web` (`zot.bex-registry.svc:5000/drill/web:1`, `imagePullPolicy: Always`, as the operator writes tenant containers) and a control `typo` (`docker.io/library/typo-image-that-does-not-exist:latest`) |
| Metrics | kube-state-metrics v2.20.0 (chart default; the pinned production chart line, prometheus 25.30.x, ships 2.14.0, and `image_spec` is in the 2.4.0 source), Service named `prometheus-kube-state-metrics` as in production |
| Prometheus | 3.13.5 (the version CI pins), scrape interval 15 s (production 60 s), loading the repository's `alerting_rules.yml` |

## What kubelet and kube-state-metrics really reported

| What | Observed |
| --- | --- |
| Kubelet state of the restarted `web` pod | `ErrImagePull`, alternating with `ImagePullBackOff`, first visible about 38 s after Zot was scaled down. Message: `failed to pull and unpack image "zot.bex-registry.svc:5000/drill/web:1": … Head "https://127.0.0.1:30500/v2/drill/web/manifests/1?ns=…": dial tcp 127.0.0.1:30500: i/o timeout` |
| `kube_pod_container_status_waiting_reason` | `reason="ErrImagePull"` and `reason="ImagePullBackOff"` as two series per container, one at 1 and one at 0 at any instant, swapping |
| `kube_pod_container_info` for `web` | `image="127.0.0.1:30500/drill/web:1"` (the runtime's alias, because the node also held the image under the tag used to push it) **and** `image_spec="zot.bex-registry.svc:5000/drill/web:1"` (the pod spec) |
| `kube_pod_container_info` for `typo` | `image` and `image_spec` both `docker.io/library/typo-image-that-does-not-exist:latest` |
| Scaled-to-zero Zot | `kube_statefulset_replicas = 0` and `kube_statefulset_status_replicas_ready = 0` |

## Timeline (UTC)

| Time | Event |
| --- | --- |
| 10:15 | `typo` begins `ImagePullBackOff` (control) |
| 10:20:09 | Zot scaled to 0, `web` pod deleted so the new pod must pull with `Always` |
| 10:20:47 | `TenantRegistryImagePullFailing` pending for `web` only |
| 10:20:09 to 10:23:26 | `ZotRegistryNotReady` (first version, shortfall only) **stays inactive** with Zot at 0 |
| 10:23:26 | rule reloaded with the "none ready" branch; pending at 10:23:32 |
| 10:28:36 | `ZotRegistryNotReady` **firing** (critical), 5 minutes after pending |
| 10:30:52 | `TenantRegistryImagePullFailing` **firing** (warning), 10 minutes after pending; labels `namespace=tea-drill`, `pod=web-…`, `container=web` |
| 10:31:11 | Zot scaled back to 1; ready at 10:31:15 (4 s; a 1 GiB PVC with one tiny image, not a measure of a production restart) |
| by 10:31:37 | `ZotRegistryNotReady` inactive |
| 10:31:51 | `web` pod pulled the image and started (kubelet backoff, 36 s after Zot returned) |
| by 10:32:08 | `TenantRegistryImagePullFailing` inactive |

The control pod `typo` sat in `ImagePullBackOff` for more than 16 minutes and **never** raised `TenantRegistryImagePullFailing`: an image on another registry is not counted.

## Findings, both fixed in the milestone

1. **`image` is not the pod spec.** The first matcher read `kube_pod_container_info{image=…}`, which here was a node-local alias, so a registry-served pod could have been missed. The rule now reads `image_spec`. A promtool case with real labels (`image` an alias, `image_spec` the registry) fails on the old matcher and passes on the new one.
2. **A StatefulSet scaled to 0 is invisible to "desired minus ready".** Desired and ready were both 0, so the shortfall was 0 and `ZotRegistryNotReady` stayed silent for the 3 minutes 17 seconds until the rule was changed (the first version would have stayed silent for the whole outage). The rule now has a second branch, `ready < 1`, with a promtool case that fails without it. Argo CD's `selfHeal` reverts a manual scale, but an outage it has not healed, or cannot, is still an outage. The Grafana panel had the same blind spot and now draws `max(desired, 1) − ready`.

## What this drill does not show

- **Production restart-to-ready of Zot.** Production Prometheus was not read for this milestone, so the 5-minute `for` on `ZotRegistryNotReady` stays an assumption: the operator's credential-activation bounce deletes the pod at most once per 2 minutes (`lego/operator/internal/registry/verify.go`), and a recreate is expected to return well inside 5 minutes. If the alert ever pages on a credential activation, read `kube_statefulset_status_replicas_ready{namespace="bex-registry"}` history and move the window.
- **Alertmanager routing.** The alerts reached `firing` in Prometheus; delivery to a receiver was not part of this drill (`scripts/alerts-verify.sh` covers the route mechanics).
- **Production scrape cadence.** 60 s scrapes add up to a minute to each `for` window.
