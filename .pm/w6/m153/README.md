# w6 · m153 — Production cluster access survives a control-plane roll

**Worker:** worker6 **Goal:** every workflow that fetches the production kubeconfig keeps working across a control-plane roll, with SSH host keys still authenticated, never trusted on first use **Status:** todo (t008: stable CP host keys, option (a) chosen 2026-10-29; t001–t006 done)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Choose how pinned control-plane host keys follow a roll (user decision) — **DONE** | 15m | — |
| t002 | Implement the chosen re-pin path — **DONE** | 60m | w6/m153/t001 |
| t003 | Fail fast and name the cause when every pinned key mismatches — **DONE** | 30m | w6/m153/t001 |
| t004 | Prove it across a real control-plane roll — **DONE** | 30m | w6/m153/t002, w6/m153/t003 |
| t008 | Make the pin follow a full roll with no human: stable CP host keys (option a, user decision 2026-10-29) | 2h | w6/m153/t004 |
| t009 | Re-prove across a full control-plane roll | 45m | w6/m153/t008 |
| t005 | Simplify — **DONE** | 15m | w6/m153/t004 |
| t006 | Test coverage — **DONE** | 20m | w6/m153/t004 |
| t007 | Closeout | 10m | w6/m153/t005, w6/m153/t006, w6/m153/t009 |

## Definition of done

- After a control-plane roll, `deploy.yml`, both restore drills, `alerting-heartbeat.yml`, `app-cluster.yml` and `disk-snapshot-drill.yml` fetch the kubeconfig with strict host-key checking, with no hand edit of `BEX_SSH_KNOWN_HOSTS`.
- A mismatch on every candidate fails with one message naming a stale pin, and how it is refreshed, instead of three generic "Host key verification failed" lines.
- `:22` stays key-only. No static source-IP allowlist (`.pm/DO_NOT_DO.md`).

## Source + Goal linkage

- **Source:** 2026-10-24 diagnosis (w6/089). The w5/174 node roll (`98d97f365`) changed control-plane IPs and host keys between 05:20 and 06:18 UTC. `disk-snapshot-drill` run 40066023322 then failed host-key verification on `178.105.18.185` (key changed), `2.28.137.211` and `2.31.79.239` (not in the pin). Nothing refreshes the pinned `BEX_SSH_KNOWN_HOSTS` secret, and the local `.env` copy is also stale (the user's local fetch failed the same way on 2026-10-23).
- **Goal linkage:** ADR019 infra credentials (authenticated admin.conf fetch, codex F10). Every production deploy and drill depends on this fetch.
- **Expected outcome:** routine control-plane rolls stop silently breaking deploys and operational drills.
- **Correction (2026-10-24 21:02 UTC):** the outage was transient. The pinned secret, unchanged since 2026-08-17, matched again once the roll settled (drill run 40132266076 fetched via `178.105.18.185`), and deploys succeeded. So the failure window is during a roll, not after it. Lower priority. The DoD still holds: a fetch must not fail mid-roll, and a stale pin must name itself.
- **Why now:** it blocks w6/089's diagnosis and w6/088 today, and every deploy until the pin is refreshed.
- **Options for t001:**
  - (a) stable control-plane host keys provisioned through the CAPH bootstrap from a secret, so the pin never changes;
  - (b) the roll workflow captures new keys over its still-authenticated kube API channel (kubeconfig from before the roll; the API LB is unchanged) and updates the secret;
  - (c) stop fetching admin.conf over SSH and use a long-lived scoped kubeconfig in CI custody.
- **Render parity:** omitted. Platform infrastructure, no tenant-facing surface.

## Decision and implementation — 2026-10-26

- **t001:** the user deferred the design choice ("continue"). The 21:02 UTC correction showed the outage was transient (mid-roll), so the shipped fix is a bounded retry in `fetch-app-kubeconfig.sh`, not options (a)–(c). It rediscovers the control planes and retries only on host-key failures (default 6 × 60 s), then names a stale pin. Option (a), stable host keys, would roll the production control plane and stays a separate owner-approved change, recorded in ADR019 § Mid-roll retry.
- **t002/t003/t006:** `scripts/fetch-app-kubeconfig.test.sh` runs in `scripts.yml` with stubbed curl/ssh/kubectl. It covers a roll that settles after two attempts, a stale pin that fails after the bounded attempts with the named error, and a refused connection that gets no retry and no stale-pin claim. `github-actions-validate` passes.
- **t005:** a small, self-reviewed diff. The shellcheck CI list names only the test, because naming the fetcher there trips the admin.conf-fetcher pin guard.
- **Remaining (t004):** observe a deploy or drill across the next real control-plane roll.

## Decision — 2026-10-29 (deliberate control-plane roll authorized)

- **IDs:** w6/m153/t004, t007. **Source:** explicit user decision in `/unblock w6` (D2 = deliberately roll the production control plane once), on evidence re-checked at `90507afa5`.
- **Why:** no control-plane roll has happened since the retry shipped (`7bdc0bd45`, 2026-10-26). `app-cluster` runs 40255267703, 40260620920 and 40295455305 all reported `kubeadmcontrolplane/bex-control-plane unchanged`. All 3 CP Machines (`bex-control-plane-7qtwv`, `-9v27j`, `-ghhxf`) are 5d old, from the 10-24 w5/174 roll. No other open board item schedules a roll, so waiting could block this indefinitely.
- **Authorized scope:** one rollout of `kubeadmcontrolplane/bex-control-plane` (ns `bex-capi`, production, self-hosted CAPI), triggered by setting `spec.rolloutAfter` to now. Machine template, Kubernetes version and host-key provisioning are not changed. Option (a), stable host keys, stays a separate owner-approved change.
- **Gate verified 2026-10-29:** the local `~/.kube/bex-hetzner.kubeconfig` reaches the API. `kubectl auth can-i patch kubeadmcontrolplanes -A` = `yes`. KCP is 3/3 ready, available and up to date, not paused.
- **Guardrails:** start only when KCP is 3/3 Ready and no deploy is mid-rollout. Abort (do not patch further) if a replacement Machine stays unready for more than 20 min. Do not hand-edit `BEX_SSH_KNOWN_HOSTS` during the window, because that would void the proof.
- **Next (worker):** follow t004's steps. Acceptance: run ids recorded here for a `deploy.yml` (or `workflow_dispatch` of `app-cluster.yml`) run and a `disk-snapshot-drill.yml` run that fetched the kubeconfig during or right after the roll with strict host-key checking. Record any retry or stale-pin lines from their logs. Then t007 closes the milestone if the DoD holds. If a fetch fails because the pin is stale after the roll settles, that is a real finding: record it and keep m153 open; it is not a pass.

## t004 result — 2026-10-29 (full roll: mid-roll passes, post-roll pin goes stale)

A deliberate `spec.rollout.after` roll of `kubeadmcontrolplane/bex-control-plane` (v1beta2 field; `spec.rolloutAfter` is ignored) ran 07:40:42–~08:05 UTC and replaced **all three** CP Machines: `7qtwv`/`9v27j`/`ghhxf` (2.28.137.211 / 2.31.79.239 / 178.105.18.185) → `btjrv`/`sdffv`/`kt5cz` (91.107.197.144 / 2.31.79.239 [IP reused, new key] / 178.105.109.4).

- **Mid-roll, PASS:** `deploy.yml` run 40829651098 (07:52:25, `4128bfe0a`, success) and `disk-snapshot-drill.yml` run 40838565842 (07:56:49, success) each refused the unpinned new node 91.107.197.144 under strict checking, then fetched via a still-pinned old node (178.105.18.185). Run 40838298211 (07:53) also fetched mid-roll. It failed later in diagnose only because its target service (`srv-dbcd1t4iimkc73f6l0m0`) no longer exists.
- **Stale pin named, PASS (DoD 2):** after the roll settled, drill run 40839917848 failed all 6 attempts with the single named message `control-plane host keys still do not match the pinned BEX_SSH_KNOWN_HOSTS after 6 attempt(s) … re-pin …`.
- **Post-roll, FAIL (DoD 1):** once no old node remains, every fetch fails until the pin is re-captured. The 2026-10-24 "pin matched again once the roll settled" correction held only because that roll left `ghhxf` (178.105.18.185) in place. A full roll breaks every deploy and drill.
- **Recovery (hand re-pin, which DoD 1 forbids as routine):** the new ed25519 keys were read from `/etc/ssh/ssh_host_ed25519_key.pub` on each node through the authenticated kube API (a short-lived busybox pod per node in `kube-system`, then deleted). Their fingerprints matched an independent laptop `ssh-keyscan`: `+2Tu0MXz…` 91.107.197.144, `ptclu1uu…` 178.105.109.4, `2wy4fYom…` 2.31.79.239. `BEX_SSH_KNOWN_HOSTS` was set in `production-deploy`, `-cluster`, `-restore` and `-observe` at 08:16 UTC. Drill run 40840971399 then fetched via 178.105.109.4 (success). This checkout's `.env` carries no `BEX_SSH_KNOWN_HOSTS`, so it needed no edit.
- **Also found:** the first replacement Machine (`r2nmn`) hit `ServerCreateFailedIrrecoverableError` (Hetzner `resource_unavailable` for cx33 in fsn1). The roll stalled until the Machine was deleted by hand, and the retry (`sdffv`) succeeded in <2 min. Filed as w6/098.
- **Related same-day change:** `production-restore`'s leftover required reviewers (which parked the first drill, 40837155767) were removed by user decision. `scripts/github-environments-drift.sh` now reports any manual gate.

## Gate — 2026-10-29

DoD 1 needs the pin to follow a full roll with no human. The retry cannot do that. The remaining options are those of t001: (a) stable CP host keys provisioned through the KCP bootstrap from an in-cluster secret, so the pin never changes; (b) automated re-pin over the authenticated kube API (what was done by hand above), which needs a new GitHub credential able to write environment secrets; (c) a CI-custodied kubeconfig. The 2026-10-26 record reserves (a) as an owner-approved change, so the milestone waits on that choice (t008).

## Decision — 2026-10-29 (t008: stable control-plane host keys)

- **IDs:** w6/m153/t008, t009. **Source:** explicit user decision in `/unblock w6` (D3 = option (a)).
- **Chosen:** option (a). One ed25519 CP host key lives in an in-cluster Secret in `bex-capi`, and the KCP `kubeadmConfigSpec` writes it to `/etc/ssh/ssh_host_ed25519_key{,.pub}` (`files` with `contentFrom.secret`, then an sshd restart in `preKubeadmCommands`) on every new CP node. The pin then never changes across rolls. Not chosen: (b), which needs a new GitHub credential with secret-write; (c), which puts a long-lived cluster credential in CI.
- **Scope:** CP nodes only. The KCP spec change rolls the control plane once (authorized by the same decision, under the t004 guardrails). After that roll, `BEX_SSH_KNOWN_HOSTS` holds the single stable key for the CP addresses. Workers are out of scope.
- **Acceptance:** t009 — a full deliberate roll after t008 ships, with fetches mid-roll and after the roll, and no pin edit.
