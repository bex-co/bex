# w1 · m87 — Persistent disks: make the running system match its records

**Worker:** worker1 **Goal:** every claim ADR018 and ADR082 make about persistent disks becomes true of production, not just of the code. **Status:** done

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Provision the disk-snapshot bucket + a disk-dedicated age keypair | 45m | — | — **DONE**
| t002 | Wire `BEX_DISK_SNAPSHOT_*` into the operator and bex-api manifests | 45m | t001 | — **DONE**
| t003 | Mirror the new variable names into `.env.example` | 15m | t002 | — **DONE**
| t004 | Verify a real snapshot end to end on a live disk | 1h | t002 | — **DONE**
| t005 | Say "not configured" instead of "internal error" on the Snapshots card | 30m | — | — **DONE**
| t006 | Verify the disk usage series against real Prometheus | 45m | — | — **DONE**
| t007 | Render parity check | 30m | t004, t005, t006 | — **DONE**
| t008 | Simplify pass | 30m | t007 | — **DONE**
| t009 | Test coverage | 45m | t007 | — **DONE**
| t010 | Closeout | 30m | t009 | — **DONE**

## Progress notes

**t001–t003 done as CODE, not as a manual procedure (2026-08-24).** The original block was "I cannot create a bucket or a keypair" — but the repo's own convention is that a *script* creates them (`scripts/agent-snapshot-secret.sh`, `scripts/backup-s3-credentials.sh`). So the deliverable was the provisioner, not the credentials:

- `scripts/disk-snapshot-secret.sh` — `provision` / `verify` / `install`, modeled on the agent-snapshot script with two deliberate differences. **Two identities, not one**: the operator writes and deletes (backup, retention, purge-on-detach) while bex-api only LISTS, so bex-api can never write or delete a tenant's backups and never holds the age key. And an age keypair **dedicated to disks**, not ADR050's platform key, because a restore must decrypt inside the cluster and ADR050 exists to keep the platform key out of it.
- `infra/wasabi/disk-snapshot-s3-policy.json` + `-read-s3-policy.json` — the two least-privilege policies, each pinned to the dedicated bucket.
- `scripts/disk-snapshot-secret.test.sh` — 19 offline guards, wired into `.github/workflows/scripts.yml`. They cover the refusals that matter: a shared bucket, a bucket the committed policies do not name, an unpinned `aws-cli` image, a missing credential, and **any policy edit that would hand bex-api PutObject or DeleteObject** — the mistake the two-identity split exists to prevent, which no Go test would catch.
- Operator + bex-api manifests wired, both `optional: true` so a cluster without the Secrets is byte-identical to before the feature existed. Env names cross-checked against what the Go code actually reads: zero drift in either direction, and `.env.example` covers every name.
- `docs/runbooks/disk-snapshot-setup.md` — the procedure, its rollback, and the age-rotation hazard (rotating without keeping the old key makes every existing snapshot permanently unreadable).

Two bugs caught in my own work before shipping: `age-keygen -o` refuses to overwrite, so `mktemp` (which creates the file) made the fallback fail — now a 0700 directory with a not-yet-existing path, and the exact sequence is verified. And the `verify` subcommand originally only proved the credentials worked; it now proves the *separation* holds, which is the part the design depends on.

**t007 (parity) done.** The not-configured answer is one answer on all three surfaces, and a permanent guard pins it: the verb returns a `CodedError` whose `Extensions()["code"]` GraphQL carries, whose `ErrUnavailable` class REST maps to 503, and which `IsPublicError` accepts so GraphQL does not flatten it to "internal error" — the exact failure that put those words in front of tenants. Both snapshot verbs are checked, not just the one that was looked at.

**t008 (simplify) done, with one thing deliberately left alone.** The provisioner already avoids its own obvious duplication: `ensure_user_and_keys` is parameterized over (user, policy, out-vars) and serves both identities rather than being copy-pasted. Its aws-cli wrapper helpers (~40 lines) do duplicate `scripts/agent-snapshot-secret.sh`, and `scripts/lib/` exists as a home for shared shell — but extracting them would mean editing a working, shipped script that this milestone did not otherwise touch, and two instances is below the threshold where the extraction pays. Worth doing when a third provisioner appears.

**t009 (coverage) done.** 19 offline guards on the provisioner's refusals (CI-wired), two backend tests on the coded error and its cross-surface identity, and two dashboard tests separating "not configured" from a real outage. Each asserts a failure mode that actually occurred or would silently regress; none is a snapshot-everything test.

**t004 done — the drill passed end to end on the CAPD cluster (2026-08-25).** The literal DoD sequence: a disk-bearing App produced a `Bound` 10 GiB PVC and its LUKS Secret; the operator created the backup CronJob (`45 2 * * *`); a triggered run wrote a real object at `disks/<ws>/<app>/2026-08-25T06:55:44Z.tar.gz.age`; the disk was mutated (marker flipped to POST-SNAPSHOT, a second file added) and restored, after which the marker read **PRE-SNAPSHOT** and the added file was **gone**; detaching ran the purge Job, leaving **zero** objects and no CronJob.

**The drill earned its keep three times over — every defect was in the m87 work itself, and each failed LATE:**

1. The operator's Secret needs `AWS_ACCESS_KEY_ID`/`AWS_SECRET_ACCESS_KEY`, not `BEX_DISK_SNAPSHOT_*` — the Job `envFrom`s the whole Secret into the AWS SDK. Wrong names fail at upload with the misleading `no EC2 IMDS role found`, i.e. only once a backup is actually attempted.
2. That Secret must exist in **every tenant namespace**. The Job runs beside its App (ADR043 D8) and nothing projects it the way `BackupSourceNamespace` projects the KeyValue credential; `bex-system` alone leaves every Job in `CreateContainerConfigError`.
3. The age Secret must hold the **bare key**, not the file `age-keygen` writes — its comment lines produce `parse identity: malformed secret key: mixed case`, discovered only when restoring a snapshot that had already been taken. That is the worst failure mode of the three: backups appear to work and are unrestorable.

All three are fixed in the script and pinned by three new guards (22 total, CI-wired). Two further contract details are now documented: `restoreSnapshot` takes the full `<ws>/<app>/<file>` key, and the per-App restore Job is not recreated while it exists.

**Also confirmed working as designed:** the `SnapshotImageUnresolved` fail-closed guard (refused to schedule rather than create a broken CronJob), and ADR043's tenant egress policy, which blocks RFC1918 destinations — so a self-hosted store on a private network needs an explicit allowance. Both are noted in the runbook.

**Environment repairs required to get there** (the disk had filled to 96%, taking the cluster with it): 55 GB reclaimed, Docker restarted, the destroyed `kube-apiserver` serving cert regenerated from the surviving CA, kubelets and kube-proxy repointed off the wedged CAPD load balancer, and Calico restored. and it genuinely needs a live run: attach a scratch disk, await the CronJob, confirm one object lands, list it, restore it, delete and confirm the purge. The runbook has the literal sequence.

**t006 answered, no code change needed (2026-08-24).** The "No data in range" chart was kubelet volume-stats warm-up on a freshly attached disk, not a broken matcher — the task named both possibilities and required evidence rather than an assumption. Queried the live API for the same metric on a database disk and the service disk: both return real series, and the service series carries `instance=disk-tea-da2isimlm39c739m4ofg-beancount-ledger`, i.e. exactly the claim `appv1alpha1.DiskPVCName` derives. The chart now renders. The `kind="service"` path is correct.

That diagnostic surfaced a **pricing defect filed as [078](../078.md)**: the live 1 GB disk sits on a ~10 GiB volume because Hetzner's volume minimum is 10 GB — which ADR082's own context section records — while `diskMinSizeGB = 1` and the UI ships 1 GB and 5 GB chips. bex charges $0.175 and pays ~$0.50 for a 1 GB disk. It needs a product decision, so it is a separate note rather than folded in here.

## Definition of done

A disk-bearing service on production has a backup CronJob; at least one real snapshot object exists under its prefix; `GET /v1/disks/{diskId}/snapshots` lists it; a restore on a scratch disk returns the pre-snapshot bytes and discards the post-snapshot ones; deleting that disk purges its objects. The Snapshots card never renders "internal error" for an unconfigured store, and a genuine failure stays visibly distinct from "never configured". The Disk usage chart either shows a real series or its absence is explained by a Prometheus query proving the series does not exist yet. `.env.example` carries every new name.

## Source + Goal linkage

- **Source:** [.pm/w1/077.md](../done/077.md) (snapshots ship inert) plus two defects found in the 2026-08-24 production walkthrough of the w1/m86 Disk tab, neither of which had a note.
- **Goal linkage:** ADR018 ledger truthfulness and ADR082 D5. The disk row now claims ✅ on all four surfaces and advertises daily snapshots with seven-day retention; that is currently true of the code and false of the running system. A parity ledger that overstates is worse than one that admits a gap.
- **Expected outcome:** a tenant who attaches a disk actually gets backups, and the UI tells the truth when they are unavailable.
- **Why now:** m83–m86 shipped the whole disk feature; this is the gap between "shipped" and "working in production", and it is cheapest to close while the design is fresh. The two UI defects were found by driving the real page — the unit suites were green throughout, which is exactly why they need regression tests.
- **Render parity task included** (t007): the milestone changes tenant-facing snapshot availability, its error states, and the usage chart.
