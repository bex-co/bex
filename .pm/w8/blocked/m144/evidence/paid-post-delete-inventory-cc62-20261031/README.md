# Original inventory completed after fixture deletion — 2026-10-31

The original read-only [run 41197243038](https://github.com/bex-co/bex/actions/runs/41197243038), cluster job `124526099401`, completed successfully at exact source `fa85c44184ffa59879b8c1a77c991e3ce87fa028`. The cluster job ran from 13:07:13 to 13:09:20 UTC. Its inventory JSON began at 13:09:14.9653586 UTC, about 32 minutes after the fixture's single DELETE returned 204.

[Selected facts](inventory-selection.json), [job metadata](job-selection.json) and [custody](CUSTODY.json) bind this observation to the actual completed job. The full 35,315-byte log was fetched once and remains private, mode 0600, SHA-256 `dd1c65f4dec38dd8c91e9e430c3c9ecea9206d72cfcc4b5feb9d62a3fd118792`. Nonfixture tenant names, UIDs and volume handles are omitted from this export. The exact inventory source files at the job commit are retained for scope review.

The report contains two existing requested-10-GB disk-bearing Apps, two App-controller-owned PVC rows and zero unmatched App-controller claims. It reports no disk-bearing App with the original service ID `srv-dbit2iat5fis7381jscg`, no disk-bearing App in workspace `tea-da2isimlm39c739m4ofg`, and no App-controller PVC in that workspace. Disk ID `dsk-dbit2iat5fis7381jsdg` is not an inventory projection field.

These are post-delete selected metadata observations. The App projection filters on `spec.disk`; the PVC report retains only App controller ownership; PV reads cover only names referenced by those current claims. An ownerless PVC or an orphan PV can therefore fall outside the report. It does not establish all-kind App absence or the exact former App/PVC/PV lifetimes. The original fixture's UIDs and provider handle were never captured in a pre-delete binding.

LUKS Secret, backup/purge workload and VolumeAttachment absence, exact provider-volume absence and S3 purge remain **unproved**. No provider-volume baseline, purge watch, object-store read or UID-scoped cleanup observer was run. This late report cannot supply those missing observations retrospectively.

The [shipped ended-attempt evidence](../paid-attempt-cc62-20261031/README.md) remains unchanged: API 204/404s, baseline quota, own logout and watchdog completion are retained there. Hosted full/grow/remount/backup/restore, complete surfaces and requested-size billing acceptance remain open. This follow-up used no QA reauthentication, fixture operation, SSH retry or new workflow dispatch.
