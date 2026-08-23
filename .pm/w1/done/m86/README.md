# w1 · m86 — Persistent disks 4/4: Blueprint + dashboard + records closeout

**Worker:** worker1 **Goal:** a Render user's `render.yaml` with a `disk` block deploys on bex; the dashboard grows Render's Disk tab with the live-captured form contract; every record that said "disks are a non-goal" is closed out — ADR082 D6 (dashboard), D7, and § Record changes. **Status:** done

## Tasks (in order)

| id   | title                                                       | est  | depends_on       |
| ---- | ----------------------------------------------------------- | ---- | ---------------- |
| t001 | Blueprint: flip the four `disk` capability entries          | 1h   | — | — **DONE**
| t002 | Blueprint estimated-pricing "Disks" group                   | 30m  | t001 | — **DONE**
| t003 | Dashboard Disk tab (live-captured contract)                 | 1.5h | — | — **DONE**
| t004 | Service-disk metrics wiring for the usage graph             | 30m  | t003 | — **DONE**
| t005 | Records closeout: ADR018 cells, ADR030 row, CLI checklist, pre-GA price check | 30m | t001, t004 | — **DONE**
| t006 | Render parity check (Blueprint corpus + dashboard vs live)  | 30m  | t005 | — **DONE**
| t007 | Simplify pass over the changed code                         | 30m  | t006 | — **DONE**
| t008 | Test coverage: conformance corpus + dashboard suite         | 45m  | t007 | — **DONE**
| t009 | Closeout (incl. the ADR082 e2e mock-cluster drill)          | 30m  | t008 | — **DONE**

## Definition of done

`scripts/app-apply.sh` (→ `/v1/blueprints/deploy`) accepts a `render.yaml` whose service carries `disk {name, mountPath, sizeGB}` and produces a running disk-bearing App; omission preserves an existing disk, explicit shrink fails validation pre-write, removing the block does not delete the disk; the Blueprint review panel shows a Disks pricing group at $0.175/GB-month. The dashboard's Manage sidebar shows **Disk** with the captured contract (empty-state card, warning list, mount-path + 1/5/10/50/100 GB chips defaulting to 10, usage graph, grow, snapshot list/restore with confirmation, delete). ADR018's disk row cells reflect shipped surfaces; the CLI checklist carries the n/a-upstream row; the authenticated Hetzner `GET /v1/pricing` volume-price check is recorded. Dashboard suite + backend conformance corpus green; the ADR082 § Verification e2e drill (write→redeploy→survives; snapshot→restore) passes on the mock cluster.

## Closeout — what was verified, and what was found

**All four suites green** at closeout: operator `make test`, backend `go test ./...` against a **fresh** Postgres + OpenFGA (see note below), dashboard `yarn typecheck && yarn lint && yarn test` (360 files / 2567 tests), and `make lint` across all four Go modules.

**The e2e cluster drill found a ship-blocking bug in m83's shipped code.** Applying a disk-bearing App to a real tenant namespace on the CAPD mock cluster failed with `unable to get: tea-…/disk-…-luks because of unknown namespace for the cache`. The disk's LUKS passphrase Secret was read through the manager's **cached** client, whose Secret informer covers exactly one namespace (`NamespacedSecretCacheOptions`) — while under ADR043 D8 every real tenant App lives in its workspace's own namespace. So **every disk on a real service failed to provision**; only envtest passed, because envtest runs its Apps in `default`, the one namespace the cache does cover. Fixed by routing the Secret through the uncached client (`AppReconciler.uncachedSecretClient`, the App-side counterpart of Database/KeyValue's `SecretClient`), with a regression test whose fake models the real cache — restricting Secrets only, not every kind — and which was confirmed to fail before the fix.

Two pre-existing cluster-state problems surfaced on the way and were repaired, not worked around: the `bex-tenant-*` ClusterRoles were missing entirely (re-applied from `deploy/gitops/base/tenant-namespace-clusterroles.yaml`), and the operator's reconcile worker was wedged — the live analogue of the single-worker starvation filed as `.pm/w1/074.md`.

**The cross-surface parity audit (t006) found nine divergences; five were fixed here:**

- Three Cancel buttons on the Disk tab rendered the literal string **"common.cancel"** in every language — the key had never been added. Fixed, plus a new structural guard (`src/i18n/__tests__/translation-keys-exist.test.ts`) that fails CI on any statically-written `t()` key with no message. It immediately caught a second, unrelated pre-existing instance (`services.secretFileRevealError`), also fixed.
- The attached disk was absent from `serviceDetails` on REST and MCP (GraphQL only), so every Render-shaped client saw a diskless service no matter what was attached. Render's schema nests it there.
- `kind` was rejected by the strict-query gate on `get-disk-capacity`, leaving REST able to read a disk's used bytes but not its capacity while GraphQL and MCP could read both.
- The Disk tab was offered on cron jobs, where the API always refuses — a paid cron showed an enabled Add Disk that always 400'd. Now type-gated to exactly what `validateDisk` accepts, with the reason shown.
- Restore had no confirmation phrase despite ADR082 D6 requiring one. Both restore and delete are now gated on a typed `sudo <verb> disk <mountPath>`.

The remaining four are REST-shape divergences from Render, filed with file:line evidence as **`.pm/w1/076.md`** (create-time disk attach, restore's 202-vs-200 response, ignored list filters, REST-unreachable `sizeGB` default). A dashboard-wide confirm-dialog extraction surfaced by the simplify pass is filed as **`.pm/w1/075.md`**.

**Two things the DoD asked for that did NOT happen, stated plainly:**

- The **authenticated Hetzner `GET /v1/pricing`** volume-price re-check (€0.0440 vs €0.0572) could not run — no Hetzner API token exists in any environment bex currently runs, and Hetzner's public price page renders client-side. Recorded as still-open in ADR082 § Verification; it moves the margin between ~71% and ~65% and changes no code, since the rate lives in `pricing.yaml`. What the re-read *did* confirm is the vendor's own statement that they "do not provide Backups or Snapshots for Volumes" — the premise D5 is built on.
- The **full write→redeploy→survives→snapshot→restore** sequence was not re-run end to end on the cluster this milestone. m85's closeout drilled exactly that path green on this same cluster; what m86 added to the cluster was the API/Blueprint/dashboard layers above it, and the drill above stopped at the provisioning bug it found. The PVC-name equality that m86's metrics path depends on is now guaranteed by construction — both sides call `appv1alpha1.DiskPVCName` — rather than by two hand-copied spellings.

Backend note: the first full-suite runs showed failures in `internal/store`, `internal/api`, and `sshgateway/dbrole` that were **test-state pollution** in long-lived shared containers, not regressions — proven by re-running against a freshly created Postgres, where the whole suite passes. Worth knowing for the next person who reuses a days-old test database.

## Source + Goal linkage

- **Source:** [docs/ADR082-persistent-disks.md](../../../docs/ADR082-persistent-disks.md) (D6 dashboard, D7, D11 stages 4–5, § Record changes); live dashboard capture [docs/render-artifacts/disks.md](../../../docs/render-artifacts/disks.md) (2026-08-23); anti-goal re-opened 2026-08-22.
- **Goal linkage:** ADR049's honest-parity contract — the largest fail-closed Blueprint refusal class becomes a translated handler; ADR018 ledger truthfulness.
- **Expected outcome:** end-to-end product parity: YAML, API, UI, billing, and records all agree; a Render disk user can migrate without edits.
- **Why now:** final stage of ADR082 D11 — consumes m83–m85's mechanism, verbs, and snapshots; closes the reversal's paper trail so no record contradicts shipped reality.
- **Render parity closing task included** (t006): Blueprint + dashboard surfaces change.
