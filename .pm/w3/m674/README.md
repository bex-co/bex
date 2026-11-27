# w3 · m674 — Changing a Postgres's or a Key Value's plan or storage first reviews Render's line items and total

**Worker:** worker3 **Goal:** changing a Postgres database's compute plan or storage, or a Key Value instance's plan, opens Render's review before anything applies. The review lists each billed line (compute plan, high availability, read replicas, storage) with its price, then the monthly total, Render's downtime warning and Render's button ("Update compute", "Update storage", "Upgrade compute"). **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](t001.md) | Record Render's review dialog and the bex price behind each line | 30m | — |
| [t002](t002.md) | One review dialog: lead, priced lines with Edit, total, warning, confirm, error | 1h | t001 |
| [t003](t003.md) | A Postgres's compute change and storage change open the review | 1h15m | t002 |
| [t004](t004.md) | A Key Value's compute change opens the review | 30m | t002 |
| [t005](t005.md) | Verify locally with an HA database and a replica, and on production | 45m | t003, t004 |
| [t006](t006.md) | Render parity | 20m | t005 |
| [t007](t007.md) | Simplify | 15m | t006 |
| [t008](t008.md) | Test coverage | 40m | t006 |
| [t009](t009.md) | Closeout | 10m | t007, t008 |

Total: **9 tasks, about 5h25m**. **Starts after w3/m649 closes:** t004 changes the Key Value plan card that m649 moves onto the new Info page (`#compute`).

## What Render does

- **Evidence:** Render's public dashboard bundle, read 2026-11-25 without sign-in (`index-D5V4ZgbE.js`, sha256 `8d15889ace2d…`). One review component (`BN` in that build) serves every compute change; find it by its strings, since chunk names change with every deploy.
- **Postgres compute change:** lead "Review your compute and storage selections."; lines Compute plan, High availability, Read replicas and Storage, each with its price and an Edit link back to the choice; a Total; then "Your database[ and its replicas] will be unavailable for a few seconds|minutes while updating." (seconds when HA is on, minutes otherwise); button "Update compute".
- **Postgres storage change:** lead "Review your storage selection."; the Storage line and Total; button "Update storage".
- **Key Value compute change:** lead "Review your compute selection."; Compute plan and Total; a warning that the instance is unavailable while updating, plus, from a Free instance, that all existing data will be deleted; button "Upgrade compute".
- **Arithmetic** (`WP` in that build): billed instances = 1 + (HA ? 1 : 0) + replica count; the HA line is the plan price times Render's HA factor; read replicas are count × plan price; storage is GB × the per-GB rate × billed instances.
- **Same component elsewhere:** services' and cron jobs' compute changes use it too. They are outside this milestone; t006 files them.

## bex today

Verified 2026-11-25 on main `c196617a8b`.

- A Postgres plan change confirms through the generic `ConfirmDialog` (`database-plan-section.tsx`): "Change to {name}?" (`databases.planPickerConfirmTitle`), a restart sentence (`planPickerConfirmBody`) and "The new plan costs ${price}/month." (`planPickerConfirmPrice`, the plan's `monthlyUsd`). For an HA database, or one with replicas, that names a fraction of what the change costs: [ADR030](../../../docs/ADR030-pricing.md) meters an HA standby like the primary.
- A Postgres storage change applies on Save with no review (`useUpdateDatabaseDiskSize` in the same file).
- A Key Value plan change uses the same `ConfirmDialog` pattern (`key-value-plan-section.tsx`, `keyvalue.planPickerConfirm*`, with the data-loss sentence `keyvalue.restartConfirmDataLoss`).
- Prices: plan `monthlyUsd` from the plan catalogue; the HA line already shows in `ha-panel.tsx` (`databases.haEnablePrice`); Postgres storage is $0.21/GB-month (ADR030's table). ADR030 also records that bex meters Postgres storage on used bytes while its estimates price the provisioned floor, so the review's storage line is an estimate, like every other bex storage estimate.

## Definition of done

1. Locally, an HA Postgres with one read replica changing plan opens the review. It shows the Compute plan, High availability, Read replicas and Storage lines priced from bex's catalogue, and a Total equal to their sum. It shows the "few seconds" warning that names the replicas. Update compute applies the plan, and each Edit returns to the picker with the choice kept.
2. Changing only the storage opens "Review your storage selection." with its line, the Total and Update storage; nothing applies before it is pressed.
3. A Key Value plan change opens "Review your compute selection." with Upgrade compute; leaving Free adds the data-loss sentence.
4. Unit tests pin the arithmetic for no HA, HA, HA with replicas and a storage-only change, using bex's price sources, not literals copied into the test.
5. On production, once a pin carries the milestone, the review opens and cancels for a Postgres and a Key Value instance in `bex-canary` (`tea-daif693dqjvc73e7as3g`), each created for the walk (disposable), at 1440 and 390.
6. A follow-up for services' and cron jobs' compute change is filed.

## Source + Goal linkage

- **Source:** `/pm-brainstorm one aspect of render parity for w3` (2026-11-25), item 4. Before filing it was checked against Render's bundle and bex's code (the dialogs above, ADR030's HA metering). The brainstorm's conflict check against w1/m645–m648 found none: w1/m646 rewrites the rollback dialog in `deploy-actions.tsx`, which this milestone does not touch.
- **Goal linkage:** [ADR008 § Render parity first](../../../docs/ADR008-vision.md#render-parity-first), and billing honesty: the user should see what a change costs before agreeing to it.
- **Expected outcome:** someone changing a database's plan sees the monthly total of every instance it bills before confirming, as on Render, instead of the single plan price.
- **Why now:** HA and read replicas multiply a plan's price, so today's dialog understates the bill exactly for the customers who pay the most. The Key Value part waits for w3/m649 only because it moves the same card.
- **Render parity task:** included (t006). Dashboard only. REST, GraphQL and MCP plan changes do not change, and t006 confirms that.
