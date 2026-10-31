# w8 · m144 — Enforce persistent disk capacity below the provider floor

**Worker:** worker8 **Goal:** a requested 1 GB disk fills at 1 GB while preserving requested-size billing and safe grow-only resize. **Status:** todo — t001/t002 done; API/operator and key-guarded CSI verified Ready; owned growth/remount, backup/restore, billing and surface acceptance remain

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

## New CSI guard image qualified and pinned — 2026-10-30

[The new factory and independent artifact review](evidence/published-csi-key-guard-20261030/README.md) qualify `e9516afe…` at exact key-refusal source `15ed18b94…`, with the real kernel test, strict signature/source checks and successful anonymous executions on both architectures. The signed SBOM covers amd64 only, stated explicitly. Both controller/node source pins now select the new image. Its scoped live rollout is next; the paid fixture remains gated on actual CSI adoption and the separately qualified platform release.

## Signed final-source platform candidate — 2026-10-30

[The successful-build and strict candidate proof](evidence/platform-candidate-25b-20261030/README.md) binds published index `a544bb8a2c8e068674b76c560766da6e9d67566a49f0cd86696167622771b84c` to source `25b99d8a373aedd66ec485708762a59cf5eaa408`, both required fix ancestors and all seventeen tested source blobs. Signing and all ten scan steps passed; deployment was queued in the actual snapshot. Live API/operator and new CSI readiness, current CI capacity and the original paid lifecycle remain open. No fixture or task is completed by this candidate proof.

## Actual platform and guarded CSI Ready — 2026-10-30

[The actual scoped rollout proof](evidence/live-release-ready-20261030/README.md) reports the new e951 CSI driver Ready on one controller and eleven node pods. Its pre-Helm selected inventory already shows the qualified a544 platform Ready on both API pods and the manager with exact runtime imageIDs; bounded class selection/contract and both legacy bindings/handles are preserved. The reviewed validator rejects old/terminating driver pods. Separate post-CSI inventory41107548122 is pending. Fresh queue fit and all original paid lifecycle/surface/cleanup acceptance remain open; no fixture or task closes here.


## Current paid-test queue review — 2026-10-30

[Actual completed-job timing and current inventory wait](evidence/production-queue-budget-20261030/README.md) do not yet establish that the unchanged two-hour owned fixture, observer/backup work and15-minute cleanup reserve fit. Real platform/CSI Ready evidence remains valid. No resource, create intent, release gate or armed watchdog exists, and no new idle-runner requirement is introduced. Original capacity, lifecycle, billing and surface DoDs remain open.


## Local command-failure coverage — 2026-10-30

[Two executed error-path cases](evidence/command-failure-isolation-20261030/README.md) preserve actual A/B filesystem identity, geometry and data while simulated inspection/resize commands fail; no fallback formatting or false success occurs. The exact test owner and mappings are cleaned, with a restricted tools holder temporarily retained for separate testing. Harness failures and both authorized image pulls remain explicit. No kernel fault, actual billing, hosted lifecycle or separate ambiguous-format safety result is claimed; original production acceptance remains open.


## Existing-volume format regression and completed inventory — 2026-10-31

The [post-CSI fixed inventory](evidence/post-csi-inventory-20261031/README.md) now has actual completed output: qualified a544 platform Ready, bounded default and preserved legacy bindings. Separately, [one corrected local full-Publish simulation](evidence/publish-probe-routing-20261031/README.md) qualified the original criterion failure: an ambiguous inner probe on an existing LUKS mapping requested mkfs, which was refused. Both healthy filesystems and exact owned cleanup passed; the actual regression test's failure is retained. This is a source bug to fix before acceptance, not data-loss reproduction. Production source fixes and original hosted/billing/restore/cleanup DoDs remain open.


## Publish provenance fix and regressions — 2026-10-31

[The tested source fix](evidence/publish-format-provenance-20261031/README.md) prevents ambiguous inner probes on existing recognized LUKS mappings from requesting formatting, removes the second formatting fallback, and retains filesystem-check/read-only/legacy contracts. All changed-package Linux unit tests and11 owned transition cases pass with healthyA/B preserved and cleanup0. Root and independent functional reviews are clean; root applied/reversed the exact replacement against the checksum-locked upstream archive and matched all16 resulting source files. Full image-factory tests, strict artifact qualification, reviewed pin/Ready and original hosted/billing/restore/cleanup acceptance still remain.


## Existing-volume fix image qualified and pinned — 2026-10-31

[Root-reviewed actual factory and artifact proof](evidence/published-csi-publish-provenance-20261031/README.md) binds immutable53744992 to exactdd64 source and theedc67256 bounded patch. The actual full upstream suite and real kernel full/grow/remount/restore gate pass. Original strict CSI signature/source and signed SPDX checks pass; both runtime architectures independently preserve the official base and execute all four tools anonymously, with owned cleanup verified. Both reviewed controller/node pins now select this image. The signed SPDX inventory is explicitly AMD64 only. Scoped rollout and actual Ready must be verified before the original paid fixture; hosted lifecycle/billing/restore/cleanup remain open.


## Fixed CSI live and original budget reviewed — 2026-10-31

[Actual scoped rollout](evidence/live-csi-publish-provenance-ready-20261031/README.md) succeeded at10:30UTC: approved53744992 is Ready on one controller and twelve nodes, with the reviewed validator requiring old selected pods terminated. The selected pre-Helm tenant inventory preserves the original bindings/classes; no post-Helm tenant reread is claimed. [Sanitized actual gate inputs](evidence/paid-fixture-gate-inputs-20261031/) bind fresh a544 platform readiness and the new CSI source/signature/patch. The original7200-second/~$0.03 one-Starter fixture uses a reviewed95-minute lifecycle estimate plus900-second cleanup reserve. Actual lifecycle, weighted billing, restore and exact owned cleanup remain open; queue timing is a concrete estimate rather than an SLA.

## Owned paid attempt ended before lifecycle acceptance — 2026-10-31

[Sanitized attempt and private-client evidence](evidence/paid-attempt-cc62-20261031/README.md) records the original one-Starter/one-disk HTTP 201, sole initial Live deployment, initial 1 GiB status and final original data hashes. The fixed read-only inventory remained queued in the 12:59 UTC check, and existing local access failed authoritative SSH host-pin verification. Actual App/PVC UID binding and observer readiness never became available, so fill, grow, restart, redeploy, backup and restore were not run. Original program/data and the 7,200-second, 5 GiB, estimated $0.03 caps stayed unchanged.

The attempt ended early with one exact-owned DELETE returning 204 at 12:37:11 UTC. Watchdog reconciliation proved exact service/disk 404s at 12:37:14; an independent original cleanup check reconfirmed them at 12:37:43. Baseline services and quota were preserved (used 2, terminating 0, limit 25). The watchdog exited naturally, and own OAuth/Kratos logout both succeeded. Kubernetes/provider/S3 cleanup and weighted 1→5 requested-size billing remain unproved; API absence and Blueprint pricing are not evidence for those results.

Original client failures are retained: optional create-response `deployId`, display-name-derived host, overwritten disk-resource result and undeclared REST metrics `kind`. Separate reviewed observation copies leave the original files/program/data intact; a private three-line surface correction passes seven pure checks without live execution. Offline pre-delete GraphQL/MCP/Blueprint receipts supply partial requested-1-GiB observations; they do not convert the failed original surface command into a pass.

The existing t003–t007 acceptance and milestone DoD remain open. Worker8/root coordination must obtain actual cleanup evidence where exact ownership can be established, restore an authorized strict access path or obtain the queued inventory result, and establish credible CI timing within the original budget before further hosted acceptance. This record changes no task status or milestone location and introduces no idle-runner requirement or expanded fixture budget.
