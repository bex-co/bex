# w1 · m36 — Node bring-up efficiency: baked snapshot image + trimmed provisioning

**Worker:** worker1 **Goal:** Autoscaled nodes stop downloading ~350MB from external hosts in the scale-up hot path: a baked Hetzner snapshot image carries runc/containerd/kubelet, workers stop pre-pulling control-plane images, the apt list is trimmed, and `app-cluster.yml` caches clusterctl. **Status:** in progress — implementation landed (bake recipe + overlay + CI cache + structural guard); the live bake → roll → measure (t005) + closeout (t008) are an operator step (billed Hetzner snapshot + prod worker-pool roll + `/ship`), see the runbook in [`infra/packer/README.md`](../../../infra/packer/README.md).

## Tasks (in order)

| id   | title                                                         | est | depends_on       |
| ---- | ------------------------------------------------------------- | --- | ---------------- |
| t001 | Bake a Hetzner snapshot image + `HCloudMachineTemplate.imageName` | 60m | —                | — recipe + wiring landed; **bake+provision-verify pending operator**
| t002 | Drop `kubeadm config images pull` from the worker templates   | 20m | —                | — code landed; **live provision-log verify pending operator**
| t003 | Trim the apt package list                                     | 20m | —                | — code landed; **live provision verify pending operator**
| t004 | Cache the clusterctl download in `app-cluster.yml`            | 20m | —                | — code landed; **warm-cache job-log verify pending CI run**
| t005 | Measure node-join time before/after on a real scale-up        | 30m | t001, t002, t003 | — **pending operator** (live roll)
| t006 | Simplify                                                      | 30m | t005             | — **DONE** (behavior-preserving review of the diff; changes already minimal)
| t007 | Test coverage                                                 | 30m | t005             | — **DONE** (`scripts/clusterapi-validate.sh` + `.github/workflows/clusterapi-validate.yml`, negative-tested)
| t008 | Closeout                                                      | 15m | t007             | — **pending** (DoD must hold live first)

## Definition of done

A fresh autoscaler-minted tenant node joins with no runc/containerd/kubelet download from github.com/pkgs.k8s.io (verified from the node's provision log), no CP image pre-pull, and the trimmed apt set; node-join time is measured before/after and recorded on this README; `app-cluster.yml` reuses a cached clusterctl. The rolled worker pools come back healthy with tenant Apps pullable (the w1/m26 property this roll depends on stays true).

## What landed (code) vs. what's operator-gated

**Landed in the repo (this branch, uncommitted pending `/ship`):**

- **Bake recipe (t001):** [`infra/packer/bex-worker.pkr.hcl`](../../../infra/packer/bex-worker.pkr.hcl) — a Packer template baking runc `1.2.5` + containerd `1.7.26` + kubelet/kubeadm/kubectl `1.31.0` (byte-identical download/verify to the old `preKubeadmCommands`) into a `caph-image-name: bex-worker`-labelled snapshot. Runs in CI ([`.github/workflows/snapshot.yml`](../../../.github/workflows/snapshot.yml), `workflow_dispatch`), never a laptop. Recipe + bump procedure + roll runbook in [`infra/packer/README.md`](../../../infra/packer/README.md). Validated locally: `packer fmt`/`init`/`validate` all clean.
- **Overlay (t001/t002/t003):** worker `HCloudMachineTemplate`s rotated to `bex-tenant-0-baked` / `bex-platform-baked` at `imageName: bex-worker` (spec is immutable — CAPH v1.1.7 — so a rename + `infrastructureRef` repoint is what rolls the pool); worker `preKubeadmCommands` stripped of the runc/containerd/kubelet downloads, the `kubeadm config images pull`, and the whole boot-time apt install (socat/logrotate baked; at/jq/unzip/mtr/apt-transport-https dropped). Control-plane deliberately untouched (still `ubuntu-24.04`).
- **clusterctl cache (t004):** `actions/cache` keyed on `CLUSTERCTL_VERSION` in [`app-cluster.yml`](../../../.github/workflows/app-cluster.yml).
- **Structural guard (t007):** [`scripts/clusterapi-validate.sh`](../../../scripts/clusterapi-validate.sh) + [`.github/workflows/clusterapi-validate.yml`](../../../.github/workflows/clusterapi-validate.yml) — fails CI if a worker template drifts back to `ubuntu-24.04` or a worker `preKubeadmCommands` re-adds a download / images-pull / pkgs.k8s.io install / trimmed package. Negative-tested.

**Operator-gated (billed prod ops + `/ship`; cannot run from a laptop):** run `snapshot.yml`, verify one node provisions clean, `/ship` the overlay to roll both pools, measure, then close out. Full runbook: [`infra/packer/README.md`](../../../infra/packer/README.md) § "Ordering — bake before you roll".

## Measurements (t005 — fill on the live roll)

**Bake — DONE 2026-07-15:** snapshot `bex-worker-k8s-1.31.0` (id `408665247`, arch `x86`, labels `caph-image-name=bex-worker` / `bex.co/role=worker` / `bex.co/k8s=1.31.0`) built by `snapshot.yml` in 1m13s (baked on `cpx22` — the Intel `cx` line is create-blocked in fsn1). Push 1 (bake recipe) is on `main`; **Push 2 (the overlay flip that rolls the pools onto this snapshot) is still held** — the roll + the numbers below are pending.

Method: on a real autoscaler scale-up (or the recorded most-recent one), time **machine `Created` → node `Ready`** — `kubectl get machine <m> -o jsonpath` `.metadata.creationTimestamp` vs. the node's `Ready` condition `lastTransitionTime`. Confirm a tenant App schedules and pulls from Zot first-try on the new node.

| metric                                 | before (ubuntu-24.04 + boot downloads)                       | after (baked `bex-worker`)          |
| -------------------------------------- | ------------------------------------------------------------ | ----------------------------------- |
| machine `Created` → node `Ready`       | ~101–218 s (freshest CP/platform nodes, 2026-07-15)          | _TBD (tenant roll)_                 |
| external bytes pulled during provision | ~350 MB (runc+containerd+kubelet+CP images)                  | ~0 (binaries baked)                 |
| tenant App scheduled + pulled from Zot | n/a                                                          | _TBD (must hold)_                   |

**Finding (2026-07-15):** on Hetzner's fast datacenter network the download itself is cheap (~100–220 s created→Ready on the ubuntu path), so the baked image's headline win is **fragility, not raw speed** — node bring-up no longer depends on github.com / pkgs.k8s.io being up (the milestone's stated goal). The `after` numbers land when the tenant pool rolls.

**Roll status:** staged (user decision 2026-07-15). Push 2a rolls the **tenant** pool only; the **platform** pool stays on `ubuntu-24.04` until a monitored maintenance window (rolling it evicts CNPG / OpenBao's 2-member raft, which needs a manual unseal on reschedule).

## Source + Goal linkage

- **Source:** `.pm/FUTURE-MAYBE.md` "Node bring-up efficiency" (from the m19.1 `/simplify` review 2026-07-11) — **trigger fired**: "app images move into a pullable registry (zot wired end-to-end)" was met when `w1/m26` closed 2026-07-13 (live-verified: a fresh autoscaled tenant node pulled a git-built image first-try). Promoted per FUTURE-MAYBE's own rule via `/pm-brainstorm` round 2, 2026-07-14 (item 4).
- **Goal linkage:** GOAL #6 (add & remove physical machine) — scale-up latency and external-dependency fragility sit directly in the autoscaler hot path; every node provision currently depends on github.com and pkgs.k8s.io being up.
- **Expected outcome:** measurably faster, less fragile node bring-up; recorded before/after numbers.
- **Why now:** the trigger condition is met and the safety condition that deferred it (a `KubeadmConfigTemplate` change rolls the pools while app images lived only in node containerd) is gone — a roll is now safe. **Render parity omitted:** pure infra (CAPH templates, packer, CI) — no REST/GraphQL/MCP/UI surface.
