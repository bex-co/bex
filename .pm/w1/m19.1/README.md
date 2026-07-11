# w1 · m19.1 — 5-server interim: rescue prod + early pivot

**Worker:** worker1 **Goal:** Bring the hosted apps back online within the current Hetzner quota (5 servers) by pulling m19-t007's pivot forward: destroy `bex-infra` to free one quota slot for the first tenant node. Interim shape **1 CP + 2 platform + 1 tenant** (no infra; KCP webhook forbids 2 — even counts illegal with stacked etcd); when the quota is raised, m19 resumes by reverting two numbers in git (KCP 3, platform max 3). **Status:** in progress — t001+t002+t004+t005 done 2026-07-11 (pivot executed 01:39Z: machines self-managed, in-cluster autoscaler healed instantly, bootstrap autoscaler uninstalled, mgmt controllers zeroed, bex-infra destroyed 01:47Z, tenant node born by the in-cluster autoscaler 01:44Z). t003 done 02:05Z: BOTH workflows green on SSH-to-CP against the pivoted cluster (deploy rerun passed after the restore-artifact fix: dirty schema_migrations v6 + missing 0004 table from the old-prod dump, patched live, now 7/clean). Apps scheduled but ErrImagePull: zot died with the old cluster and App CRs carry no repo (old flow wrote built tags into spec.image) — revival = one git push per app repo (user). t006 correction: DNS cutover must happen BEFORE LE certs can issue (HTTP01 rides the new LB), not after.

## Tasks (in order)

| id   | title                                                                                         | est | depends_on |
| ---- | --------------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Align desired state to reality: KCP 3→2, platform max 3→2, tenant max 0 (all TEMP) — ghost machines cleared — **DONE** | 15m | —          |
| t002 | Pivot (m19-t007 core, pulled forward): clusterctl init + move, kill bootstrap autoscaler       — **DONE** | 30m | t001       |
| t003 | Rewrite CI cluster access (m19-t008 item, pulled forward): SSH-to-CP replaces SSH-to-infra — **DONE**     | 20m | t002       |
| t004 | Destroy `bex-infra` (definition retained); docs trust-chain + architecture update              — **DONE** | 15m | t003       |
| t005 | Tenant node birth: tenant pool min 1/max 3 (permanent) → apps Running, LE certs issue          — **DONE** | 20m | t004       |
| t006 | External cutover: DNS → `49.12.20.236` (user, Cloudflare) + serve verification; sync m19 — **DONE**       | 15m | t005       |
| t007 | Simplify — `/simplify` over what this milestone changed (overlay, workflows, docs)             | 15m | t006       |
| t008 | Test coverage — scripted assertion of the self-managed access path (no mgmt-cluster dependency)| 20m | t007       |
| t009 | Closeout — verify DoD holds, then move the milestone to `done/`                                | 10m | t008       |

## Definition of done

- prod serves users again on **5 servers**: 2 CP + 2 platform + 1 tenant, `bex-infra` destroyed (Terraform definition retained for DR).
- Pivot complete and self-managed: `kubectl get machines -n default` works **inside** the app cluster and shows all machines Running; the Argo-managed in-cluster autoscaler is healthy (pre-pivot crashloop gone); the bootstrap autoscaler is uninstalled.
- Both CI workflows green against the pivoted cluster via the new SSH-to-CP access path.
- The 3 Apps serve on `*.onbex.co` with valid LE certs; `dashboard.bex.co` serves from the new cluster (after the user's Cloudflare cutover).
- The only reverts pending for the quota raise are two numbers in git (KCP replicas 3, platform max 3) + `bao-init.sh` once openbao-1 schedules — everything else in this milestone is permanent m19 progress.

## Source + Goal linkage

- **Source:** m19 t006 op log (23:30 entry — Hetzner `resource_limit_exceeded`, 5/5 slots) + user decisions in session 2026-07-10/11: apps must not wait a week for the quota raise; tenant pool floor becomes min 1 ("the platform exists to host other people's apps"); pivot-first chosen over KCP-shrink because it keeps 2 etcd copies and does m19 work instead of throwaway changes.
- **Goal linkage:** V0 reliability + elasticity (#6) — same as m19; this is m19's t007 (whole) and slices of t006/t008, resequenced under the quota constraint. Naming: user explicitly requested `m19.1` (sub-milestone of m19) rather than the next free `mN`.
- **Render parity: omitted.** Pure infrastructure — no REST/GraphQL/MCP/UI surface change.
- **Expected outcome:** the three hosted apps serve traffic again this week on a self-managed cluster; m19's remaining scope shrinks to "revert two numbers + bao-init + t009 acceptance" once quota lands.
- **Why now:** every day of waiting is a day of prod downtime; the pivot is due anyway (m19 t007), does not need new servers, and destroying `bex-infra` is the only quota slot available without sacrificing etcd redundancy.

## Notes

- Ordering is load-bearing: CI access rewrite (t003) MUST land before infra destruction (t004) — both workflows fetch kubeconfigs via SSH-to-infra today; deleting it first breaks CI with no fallback.
- `clusterctl move` refuses provisioning machines — t001's ghost-clearing is the gate for t002.
- OpenBao stays 2/3 (uninitialized) through this milestone: nothing depends on it (env-vars never worked on old prod, t001 finding); `bao-init.sh` runs in m19 once openbao-1 fits.
- DO_NOT_DO check: the SSH-to-CP CI path keeps `:22` authentication-only (key-gated) — consistent with the recorded baseline; no source-IP allowlist.
