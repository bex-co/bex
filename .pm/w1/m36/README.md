# w1 · m36 — Node bring-up efficiency: baked snapshot image + trimmed provisioning

**Worker:** worker1 **Goal:** Autoscaled nodes stop downloading ~350MB from external hosts in the scale-up hot path: a baked Hetzner snapshot image carries runc/containerd/kubelet, workers stop pre-pulling control-plane images, the apt list is trimmed, and `app-cluster.yml` caches clusterctl. **Status:** todo

## Tasks (in order)

| id   | title                                                         | est | depends_on       |
| ---- | ------------------------------------------------------------- | --- | ---------------- |
| t001 | Bake a Hetzner snapshot image + `HCloudMachineTemplate.imageName` | 60m | —                |
| t002 | Drop `kubeadm config images pull` from the worker templates   | 20m | —                |
| t003 | Trim the apt package list                                     | 20m | —                |
| t004 | Cache the clusterctl download in `app-cluster.yml`            | 20m | —                |
| t005 | Measure node-join time before/after on a real scale-up        | 30m | t001, t002, t003 |
| t006 | Simplify                                                      | 30m | t005             |
| t007 | Test coverage                                                 | 30m | t005             |
| t008 | Closeout                                                      | 15m | t007             |

## Definition of done

A fresh autoscaler-minted tenant node joins with no runc/containerd/kubelet download from github.com/pkgs.k8s.io (verified from the node's provision log), no CP image pre-pull, and the trimmed apt set; node-join time is measured before/after and recorded on this README; `app-cluster.yml` reuses a cached clusterctl. The rolled worker pools come back healthy with tenant Apps pullable (the w1/m26 property this roll depends on stays true).

## Source + Goal linkage

- **Source:** `.pm/FUTURE-MAYBE.md` "Node bring-up efficiency" (from the m19.1 `/simplify` review 2026-07-11) — **trigger fired**: "app images move into a pullable registry (zot wired end-to-end)" was met when `w1/m26` closed 2026-07-13 (live-verified: a fresh autoscaled tenant node pulled a git-built image first-try). Promoted per FUTURE-MAYBE's own rule via `/pm-brainstorm` round 2, 2026-07-14 (item 4).
- **Goal linkage:** GOAL #6 (add & remove physical machine) — scale-up latency and external-dependency fragility sit directly in the autoscaler hot path; every node provision currently depends on github.com and pkgs.k8s.io being up.
- **Expected outcome:** measurably faster, less fragile node bring-up; recorded before/after numbers.
- **Why now:** the trigger condition is met and the safety condition that deferred it (a `KubeadmConfigTemplate` change rolls the pools while app images lived only in node containerd) is gone — a roll is now safe. **Render parity omitted:** pure infra (CAPH templates, packer, CI) — no REST/GraphQL/MCP/UI surface.
