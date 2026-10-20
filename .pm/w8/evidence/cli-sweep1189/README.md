# CLI sweep 1189 — disks are paid-gated at create (live, 2026-10-20)

Refusal-side only; nothing was created (fixture list verified empty). No new bug filed.

- A free web-service create carrying `serviceDetails.disk` → 400 `disks require a paid instance type`. The disk gate fires at create, before provisioning — the create-time counterpart of the nested-disks 404 found in sweep 1144.
