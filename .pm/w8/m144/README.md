# w8 · m144 — Enforce persistent disk capacity below the provider floor

**Worker:** worker8 **Goal:** a requested 1 GB disk fills at 1 GB while preserving requested-size billing and safe grow-only resize. **Status:** todo — t001/t002 done; signed CSI and global bounded class verified live; deletion metering/snapshot cleanup fixes and owned production acceptance remain

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [Verify a LUKS-compatible capacity mechanism](done/t001.md) — **DONE** | 45m | — |
| t002 | [Publish bounded CSI and converge logical intent](done/t002.md) — **DONE** | 80m | t001 |
| t003 | [Handle existing disks and accept real grow/remount behavior](t003.md) | 60m | t002 |
| t004 | [Render parity across every disk surface](t004.md) | 20m | t003 |
| t005 | [Simplify the capacity mechanism](t005.md) | 15m | t004 |
| t006 | [Test hard limits, growth and failure isolation](t006.md) | 30m | t004 |
| t007 | [Accept production capacity enforcement and close](t007.md) | 30m | t005, t006 |

## Definition of done

A real 1 GB disk refuses writes at its logical capacity on the existing minimum 10 GB Hetzner/LUKS volume. A 1→5 GB grow becomes writable online without data loss; remounts, snapshots and restore preserve the cap. Existing disks have a verified migration path. Kubelet used-byte reporting and every REST/GraphQL/MCP/Blueprint/dashboard capacity read remain coherent. Billing continues to use requested size; no extra cloud resource is introduced. Meaningful kernel-backed tests and the production owned-fixture walkthrough pass, with cleanup and unverified cases recorded. Source-only completion cannot close this milestone.

## Source + Goal linkage

- **Source:** [w8/221](../done/221.md), promoted 2026-10-29 because the original option 4 is multiple tasks and more than one hour; the original evidence and dated parity decision are retained.
- **Goal linkage:** ADR008 Render parity first and ADR082 D8: small disks must enforce the capacity the user requests and pays for.
- **Expected outcome:** applications receive a full-disk error at their purchased capacity; a provider minimum never silently increases their writable capacity.
- **Why now:** w8/215 accepts and bills 1–9 GB but directly mounts the full 10 GB filesystem, leaving the capacity residual observable.
- **Render parity:** included because writes, reads, grow and disk usage are tenant-facing across every authoring surface. Pricing variants are outside this milestone and are not gates.


## StorageClass preflight checkpoint — 2026-10-30

The [fixed-class inventory extension](evidence/storageclass-preflight-20261030/README.md) can now verify the actual bounded class contract before activation, including both publish/expand Secret namespace templates and explicit absence. Unit/protocol and real kubectl mock checks pass. The next read-only production inventory must supply the real class facts; this source checkpoint does not claim class adoption or complete t002/t003.


## Verified installed class, activation still pending — 2026-10-30

[Production inventory 41009352173](evidence/production-inventory-20261029/receipt-41009352173.json) confirms the nondeleting bounded class UID and every selected field, including binding/reclaim and all four Secret templates. Both original 10Gi bindings and handles are unchanged; there are no small or unmatched claims. The actual manager/API remain Ready on the signed old `2ac6fd…` source and effective legacy class. The newer platform run failed on Rekor attestation timeouts after existing retries; its second attempt is in flight.

The [strengthened activation preview](evidence/class-activation-preview-20261030/README.md) passed full GitOps validation and all three final simplify reviews. Its exact patch/log/file hashes are archived, and the detached checkout is removed. The patch is not applied to main or production: newer signed operator readiness is still required before selection changes, followed by actual class/binding convergence and owned production acceptance.


## Read-only fixture baseline refresh — 2026-10-30

The [shared preparation archive](../m91/evidence/acceptance-preparation-refresh-20261030/README.md) records a fresh own QA login and GET-only baseline: the unrelated m162 fixture is confirmed deleted, the existing service stays protected, and no owned fixture/create intent exists. The last complete baseline is 07:58:14 UTC; later TLS failures remain failed reads. Historical IDs stay in the rejection checks, and any missing prepared ID requires a fresh exact 404. No class activation, root create gate or disk lifecycle acceptance is claimed; those gates remain open.


## Bounded-class activation source checkpoint — 2026-10-30

[Actual signed Ready preflight and activation validation](evidence/class-activation-20261030/README.md) confirm the new capacity writer, both class contracts and unchanged original bindings. The production source now selects the bounded encrypted class globally for subsequent creates, with full GitOps validation and the prior three-review patch intact. Live effective-class convergence and the owned production lifecycle walkthrough remain required; no fixture or task is closed by this source checkpoint.


## Live bounded class and deletion billing checkpoint — 2026-10-30

[Post-flip inventory 41033611414](evidence/production-inventory-20261029/receipt-41033611414.json) verifies the effective bounded manager class, Ready signed platform image, preserved class contracts and both original 10Gi bindings. No owned fixture exists. Prepaid readiness review found service deletion could erase unmetered disk periods; [the retention fix and real Postgres checks](evidence/disk-billing-retention-20261030/README.md) preserve requested-size history and correct an independently reproduced lock-order race. Snapshot-purge finalization and fixed cleanup observation are still being completed before the paid lifecycle walkthrough; no original task or milestone is closed here.


## Deletion readiness checkpoint — 2026-10-30

The [final snapshot cleanup implementation](evidence/snapshot-delete-finalization-20261030/README.md) passed final full operator tests and all-module lint after three simplify reviews. The [bounded read-only cleanup observer](evidence/cleanup-observer-20261030/README.md) passed local protocol/cancellation and exact-identity tests. [The complete real-dependency backend gate](evidence/disk-billing-retention-20261030/parent-complete-backend-gate.json) also passed. Both fixes must be verified in a signed Ready production image before creating the budgeted fixture; no owned production lifecycle acceptance is claimed.


## Real API observer bootstrap correction — 2026-10-30

[An isolated real Kubernetes API check](evidence/real-api-watch-bootstrap-20261030/README.md) caught and corrected empty-list resource-version loss before any fixture was created. Final 35 unit cases and both real watch startup/teardown checks pass. Product Go files remain identical to the full operator/lint validation; actual production adoption and the original paid lifecycle acceptance are still pending.


## Private acceptance and weighted-meter preparation — 2026-10-30

[The updated private driver and21 guards](evidence/acceptance-private-gates-20261030/README.md) require actual signed/Ready deletion fixes and bound queue/cleanup timing before any create. Its fixed usage readback verifies weighted requested1→5GB-seconds and refuses physical10Gi charges; sealing/invoice claims remain unproved. The reviewed later cleanup-helper constant addition passes focused tests. No live gate, create intent or fixture exists, and original production acceptance remains open.


## Early readiness artifact channel — 2026-10-30

[The reviewed early artifact channel](evidence/early-readiness-artifact-20261030/README.md) makes the actual watch/provider readiness marker downloadable while the observer remains active; active log API access was confirmed unavailable. Three reviews and49 local cases cover supervision, cancellation, durable terminal status and strict smoke/real authority separation. The resource-free live GitHub artifact smoke is still required before relying on the channel. No paid fixture exists.

## Actual early artifact smoke — 2026-10-30

[The resource-free GitHub smoke](evidence/early-artifact-live-smoke-20261030/README.md) passed. Its marker downloaded successfully while the exact smoke job remained in progress before and after download; the production cluster job was skipped. The real readiness validator rejects the invalid smoke marker. Actual production release/readiness, queue fit and the owned disk watch/provider/lifecycle checks remain required; no paid fixture or root gate exists.

## Missing node encryption key refused — 2026-10-30

[The independently reproduced CSI request defect and final fix](evidence/csi-key-refusal-20261030/README.md) cover valid bounded capacity metadata with a missing/empty encryption passphrase. Final upstream Linux tests and separate before/after regressions pass, preserving legacy plaintext behavior. The existing deployed CSI predates this correction: a new signed factory image and verified controller/node adoption are required before creating the paid fixture. No production plaintext-storage observation or lifecycle acceptance is claimed.

## Tenant isolation and cleanup preparation verified — 2026-10-30

[Actual small local tenant/device checks](evidence/tenant-device-isolation-20261030/README.md) pass for capless and Render capabilities with ordinary-write controls, unchanged two-volume geometry/sentinels and exact owned cleanup. [The private release/cleanup gates](evidence/private-release-and-cleanup-gates-20261030/README.md) pass25 guard cases and9 watchdog tests, reject the old CSI guard source and retain the unarmed timer’s reviewed timeout/reconciliation fixes. Actual new image factories/adoption, queue fit and paid lifecycle acceptance remain open.
