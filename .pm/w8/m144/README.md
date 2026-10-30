# w8 · m144 — Enforce persistent disk capacity below the provider floor

**Worker:** worker8 **Goal:** a requested 1 GB disk fills at 1 GB while preserving requested-size billing and safe grow-only resize. **Status:** todo — t001 done; verified CSI rolled out (1 controller, 12 nodes); newer operator/class adoption and owned production fixture remain

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [Verify a LUKS-compatible capacity mechanism](done/t001.md) — **DONE** | 45m | — |
| t002 | [Publish bounded CSI and converge logical intent](t002.md) | 80m | t001 |
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
