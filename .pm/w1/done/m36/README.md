# w1 · m36 — Node bring-up efficiency: baked snapshot image + trimmed provisioning

**Worker:** worker1 **Goal:** Autoscaled nodes stop downloading ~350MB from external hosts in the scale-up hot path: a baked Hetzner snapshot image carries runc/containerd/kubelet, workers stop pre-pulling control-plane images, the apt list is trimmed, and `app-cluster.yml` caches clusterctl. **Status:** DONE (2026-07-15) — bake + **tenant pool rolled & measured live** (54 s vs ~101–218 s Ready, github/pkgs-independent, apps healthy). The milestone's autoscaler-hot-path goal is delivered on the tenant pool. The **platform-pool** roll (Push 2b) is spun out to [`w1/022`](../022.md): the pre-flight found it blocked on CNPG single-instance HA + OpenBao Shamir unseal — a separate platform-resilience effort, not this milestone's goal. See the runbook in [`infra/packer/README.md`](../../../infra/packer/README.md).

## Tasks (in order)

| id   | title                                                         | est | depends_on       |
| ---- | ------------------------------------------------------------- | --- | ---------------- |
| t001 | Bake a Hetzner snapshot image + `HCloudMachineTemplate.imageName` | 60m | —                | — **DONE** (snapshot 408665247 baked; tenant pool provisioned from it live, containerd/kubelet baked, 54 s Ready)
| t002 | Drop `kubeadm config images pull` from the worker templates   | 20m | —                | — **DONE** (tenant); platform mirrors on Push 2b
| t003 | Trim the apt package list                                     | 20m | —                | — **DONE** (tenant); platform mirrors on Push 2b
| t004 | Cache the clusterctl download in `app-cluster.yml`            | 20m | —                | — **DONE** (actions/cache keyed on `CLUSTERCTL_VERSION`; warm hit self-verifies on the next overlay push)
| t005 | Measure node-join time before/after on a real scale-up        | 30m | t001, t002, t003 | — **DONE** (tenant roll: 54 s vs ~101–218 s; app pulled from Zot after the SA pull-secret fix)
| t006 | Simplify                                                      | 30m | t005             | — **DONE** (behavior-preserving review of the diff; changes already minimal)
| t007 | Test coverage                                                 | 30m | t005             | — **DONE** (`scripts/clusterapi-validate.sh` + `.github/workflows/clusterapi-validate.yml`, negative-tested)
| t008 | Closeout                                                      | 15m | t007             | — **DONE** (tenant pool live; platform roll spun out to `w1/022`)

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

| metric                                 | before (ubuntu-24.04 + boot downloads)              | after (baked `bex-worker`)                          |
| -------------------------------------- | -------------------------------------------------- | --------------------------------------------------- |
| machine `Created` → node `Ready`       | ~101–218 s (freshest CP/platform nodes)            | **54 s** (tenant node `bex-tenant-0-8fkmg-nqskf`)   |
| external bytes pulled during provision | ~350 MB (runc+containerd+kubelet+CP images)        | ~0 (containerd 1.7.26 + kubelet 1.31.0 baked)       |
| tenant App scheduled + pulled from Zot | n/a                                                | ✅ held (after the pull-auth fix below)             |

**Result (tenant pool rolled 2026-07-15):** the baked node joined `Ready` in **54 s vs ~101–218 s** (~50–75% faster) with runc/containerd/kubelet already present (no github.com / pkgs.k8s.io download). CI (`app-cluster.yml` run 29391348299) green; the old tenant machine drained and was deleted; all four tenant apps came back `Running` on the baked node. The download was never the big time cost on Hetzner's fast network — the durable win is **fragility**: node bring-up no longer depends on github.com / pkgs.k8s.io being up (the milestone's stated goal), now also ~3× faster.

**⚠️ Latent gap the roll exposed (NOT an m36 regression — pre-existing, out of scope):** the tenant App Deployments carry **no `imagePullSecret`**, and Zot has required auth since ~2026-07-14 (`bex-registry-pull` secret, w7/m8). The old node had every image cached from before auth existed, so it never re-pulled; the **fresh** baked node had to pull and hit `authorization failed: no basic auth credentials` → the three git-built CMS apps went `ImagePullBackOff`. Any fresh tenant node (autoscaler, failure, this roll) would have hit this. **Fix applied live:** attached `bex-registry-pull` to the `default` ServiceAccount's `imagePullSecrets` (auto-injected into every pod, node-agnostic, operator-safe — the Deployments are App-CR-owned so a direct patch would revert) + recreated the stuck pods. Apps recovered. **Follow-up (needs a task):** this SA patch is live prod state not in git — declare it in `deploy/gitops` (or make the operator attach `BEX_REGISTRY_PULL_SECRET` per docs/ADR022 §Registry access control), else it's undeclared drift.

**Roll status:** staged (user decision 2026-07-15). Push 2a rolled the **tenant** pool ✅ (orphan template `bex-tenant-0` deleted). The **platform** pool stays on `ubuntu-24.04` — Push 2b.

**Platform-roll pre-flight (2026-07-15) — blocked on prep, do NOT roll casually:** a read-only assessment found the platform pool is _not_ drain-safe as-is:

- **All four CNPG Postgres clusters are single-instance and co-located on one node (`bex-platform-b5tch-lv84c`):** `hydra-db`, `kratos-db`, `openfga-db` (auth) + `bex-db` (control-plane). Each has a PDB `*-primary` **minAvailable=1 / allowed-disruptions=0**, so draining that node to roll it **blocks** on the DB evictions (or takes auth + control-plane DB down). No replica exists to fail over to.
- **OpenBao** is now a healthy **3-member raft** (quorum survives one member — better than the m19.1 2-member note), but seals are **Shamir/manual** (no auto-unseal env), so each member that reschedules onto a baked node comes up **sealed** and needs a manual unseal to rejoin (the PDB `maxUnavailable=1` will stall the roll until it does).

**Prerequisite for Push 2b (a real task, not part of m36's autoscaler-hot-path goal):** scale the four CNPG DBs to HA (≥2 instances + anti-affinity so failover covers the drain) **or** accept a planned auth/control-plane downtime window; plan OpenBao unseal (unseal each member as it reschedules, quorum held by the other two); then roll one platform node at a time. Filed as `.pm/w1/022.md`. The tenant pool already delivers m36's core value (faster, github/pkgs-independent autoscaler bring-up); the platform roll is cosmetic parity that shouldn't force an unsafe drain.

## Source + Goal linkage

- **Source:** `.pm/FUTURE-MAYBE.md` "Node bring-up efficiency" (from the m19.1 `/simplify` review 2026-07-11) — **trigger fired**: "app images move into a pullable registry (zot wired end-to-end)" was met when `w1/m26` closed 2026-07-13 (live-verified: a fresh autoscaled tenant node pulled a git-built image first-try). Promoted per FUTURE-MAYBE's own rule via `/pm-brainstorm` round 2, 2026-07-14 (item 4).
- **Goal linkage:** GOAL #6 (add & remove physical machine) — scale-up latency and external-dependency fragility sit directly in the autoscaler hot path; every node provision currently depends on github.com and pkgs.k8s.io being up.
- **Expected outcome:** measurably faster, less fragile node bring-up; recorded before/after numbers.
- **Why now:** the trigger condition is met and the safety condition that deferred it (a `KubeadmConfigTemplate` change rolls the pools while app images lived only in node containerd) is gone — a roll is now safe. **Render parity omitted:** pure infra (CAPH templates, packer, CI) — no REST/GraphQL/MCP/UI surface.
