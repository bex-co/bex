# w6 · m153 — Production cluster access survives a control-plane roll

**Worker:** worker6 **Goal:** every workflow that fetches the production kubeconfig keeps working across a control-plane roll, with SSH host keys still authenticated, never trusted on first use **Status:** blocked (t004 needs the next control-plane roll; t001–t003, t005–t006 done)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Choose how pinned control-plane host keys follow a roll (user decision) — **DONE** | 15m | — |
| t002 | Implement the chosen re-pin path — **DONE** | 60m | w6/m153/t001 |
| t003 | Fail fast and name the cause when every pinned key mismatches — **DONE** | 30m | w6/m153/t001 |
| t004 | Prove it across a real control-plane roll | 30m | w6/m153/t002, w6/m153/t003 |
| t005 | Simplify — **DONE** | 15m | w6/m153/t004 |
| t006 | Test coverage — **DONE** | 20m | w6/m153/t004 |
| t007 | Closeout | 10m | w6/m153/t005, w6/m153/t006 |

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
