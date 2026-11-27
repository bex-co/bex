# w3 · m674 — Changing a Postgres's or a Key Value's plan or storage first reviews Render's line items and total

**Worker:** worker3 **Goal:** changing a Postgres database's compute plan or storage, or a Key Value instance's plan, opens Render's review before anything applies. The review lists each billed line (compute plan, high availability, read replicas, storage) with its price, then the monthly total, Render's downtime warning and Render's button ("Update compute", "Update storage", "Upgrade compute"). **Status:** done (2026-11-27: every task done; t005 walked locally on dev-3 with an HA database and a replica, and on production pin `6ba91ca5b72f` the reviews opened and cancelled for a disposable Postgres and Key Value in `bex-canary` at 1440 and 390)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Record Render's review dialog and the bex price behind each line — **DONE** | 30m | — |
| [t002](done/t002.md) | One review dialog: lead, priced lines with Edit, total, warning, confirm, error — **DONE** | 1h | t001 |
| [t003](done/t003.md) | A Postgres's compute change and storage change open the review — **DONE** | 1h15m | t002 |
| [t004](done/t004.md) | A Key Value's compute change opens the review — **DONE** | 30m | t002 |
| [t005](done/t005.md) | Verify locally with an HA database and a replica, and on production — **DONE** | 45m | t003, t004 |
| [t006](done/t006.md) | Render parity — **DONE** | 20m | t005 |
| [t007](done/t007.md) | Simplify — **DONE** | 15m | t006 |
| [t008](done/t008.md) | Test coverage — **DONE** | 40m | t006 |
| [t009](done/t009.md) | Closeout — **DONE** | 10m | t007, t008 |

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

- A Postgres plan change confirms through the generic `ConfirmDialog` (`database-plan-section.tsx`): "Change to {name}?" (`databases.planPickerConfirmTitle`), a restart sentence (`planPickerConfirmBody`) and "The new plan costs ${price}/month." (`planPickerConfirmPrice`, the plan's `monthlyUsd`). For an HA database, or one with replicas, that names a fraction of what the change costs: [ADR030](../../../../docs/ADR030-pricing.md) meters an HA standby like the primary.
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
- **Goal linkage:** [ADR008 § Render parity first](../../../../docs/ADR008-vision.md#render-parity-first), and billing honesty: the user should see what a change costs before agreeing to it.
- **Expected outcome:** someone changing a database's plan sees the monthly total of every instance it bills before confirming, as on Render, instead of the single plan price.
- **Why now:** HA and read replicas multiply a plan's price, so today's dialog understates the bill exactly for the customers who pay the most. The Key Value part waits for w3/m649 only because it moves the same card.
- **Render parity task:** included (t006). Dashboard only. REST, GraphQL and MCP plan changes do not change, and t006 confirms that.

## Local walk (2026-11-26, t005 step 1)

bex-api and the dashboard ran from this tree on dev-3 (`scripts/dev-env.sh 3 up`). A user created for the walk made a `1c-2g` Postgres `m674-ha` with high availability, 10 GB and one read replica `reader`, and a Free Key Value `m674-kv` (both disposable, deleted after). Chromium headless through `playwright-core`, at 1440×900 and 390×900; no page error and no sideways scroll at either width.

| Point | Seen | Verdict |
| --- | --- | --- |
| Compute review (DoD 1) | `1c-2g` → Basic 4GB: "Apply changes?", "Review your compute and storage selections.", Compute plan "2 CPU 4 GB RAM" $52.50/month, High Availability "Enabled" $52.50, Read replicas "1 replica" $52.50, Storage "10 GB (x3 instances)" $6.30, Total $163.80/month (the catalogue's $52.50 × 3 + 10 × 3 × $0.21), "Your database and its replicas will be unavailable for a few seconds while updating.", Update compute | holds |
| Edits and Cancel | Edit compute plan closes the review with Basic 4GB still selected; Edit high availability closes it, puts `1c-2g` back and hands focus to the High Availability switch, in view; Cancel applies nothing | holds |
| Update compute | the plan read `basic_4gb` on REST afterwards | holds |
| Storage review (DoD 2) | Save on 15 GB opened "Review your storage selection." with Compute plan, High Availability, Read replicas and Storage "15 GB (x3 instances)" $9.45, Total $166.95/month, no warning, Update storage; nothing applied before it; Edit storage and Cancel returned to the field holding 15; Update storage left `diskSizeGB` 15 | holds |
| Key Value review (DoD 3) | Free → Starter: "Review your compute selection.", Compute plan "0.1 CPU 256 MB RAM 250 connections" $7.00/month, Total $7.00/month, "Your Key Value instance will be unavailable while updating.", Upgrade compute. With the store's default persistence that is all; with Persistence Off the warning adds "Persistence is Off, so the restart deletes every key in m674-kv. This cannot be undone." and Upgrade compute waits for `sudo clear key value m674-kv`, after which the plan read `starter` | holds, with the deviation below |

## Decisions and deviations

- **Render's storage review lists every line, not Storage alone.** The bundle's storage review (`rve`) lists Compute plan, High availability, Read replicas and Storage at the new size, then the Total, so bex does too; the README's "the Storage line and Total" was short of it.
- **A compute change prices the storage it leaves.** bex's effective storage is the larger of the database's and the plan's floor (`tiers.EffectiveStorageGB`), so a move to a plan with a larger floor prices that floor. Render's line is the current size; its plans have no floor above it.
- **The storage rate comes from bex-api.** GraphQL's `DatabaseInstanceType` gained `storageMonthlyUsdPerGB` from `pricing.Sheet.StorageMonthlyUSDPerGB`, so no price is copied into the dashboard. The HA and replica lines are the plan's `monthlyUsd`, as both meter like the primary (ADR030, ADR009).
- **Key Value data loss follows Persistence Off, not Free (DoD 3).** bex's Free tier has a disk and persistence on by default (ADR021, w6/m127), so leaving Free deletes nothing unless Persistence is Off. The warning adds bex's sentence whenever the restart empties the store, whatever the plan, and confirm waits for the `sudo` phrase every bex confirmation that deletes data asks for (dashboard AGENTS.md § Destructive confirmations).
- **The review is the dashboard's ConfirmDialog** (an alert dialog), so focus return, Escape and the phrase behave as in every other confirmation; a pending change holds Cancel, Escape and the Edits.
- **Prices print as `$52.50/month`,** as bex's plan cards do; Render prints `$55 / month`, with cents only when the amount has them.
- **t006:** REST, GraphQL and MCP plan and storage updates are unchanged, and their suites pass unmodified; the follow-up for services' and cron jobs' compute changes is [w3/132](../../132.md); ADR018's Postgres and Key Value rows say what bex does.

## Production walk (2026-11-27, t005 step 2)

Pin `6ba91ca5b72f` (`9c76d4ae72`, carries `2a8e5537f3`): the `dashboard` pod ran `bex-dashboard@sha256:08490e27a47a…` and every `bex-system` Deployment but the `tenant-headroom` placeholder `bex-operator@sha256:0ba1d0dcbf94…`, the pin's digests, read through the admin kubeconfig just before the walk. Walked 04:03–04:08Z as the QA user (`scripts/qa-login.sh`) in `bex-canary` (`tea-daif693dqjvc73e7as3g`), on two stores created over REST for it: the Postgres `qa-20261127-m674-pg` (disposable, `dpg-dc4g23a3u9qc73car760`, Basic 256MB, 1 GB, PostgreSQL 17, `oregon`, no HA, no replica) and the Free Key Value `qa-20261127-m674-kv` (disposable, `red-dc4g23a3u9qc73car770`, `oregon`). Google Chrome headless through `playwright-core`, `en-US`, at 1440×900 and 390×900. Prices against GraphQL's catalogue on the same pin: Basic 256MB `4.20`, Basic 1GB `13.30` with a 5 GB floor, `storageMonthlyUsdPerGB` `0.21`, Key Value Starter `7.00`.

| Point | Seen at 1440 and at 390 | Verdict |
| --- | --- | --- |
| Compute review (DoD 5) | Basic 256MB → Basic 1GB, Save: "Apply changes?", "Review your compute and storage selections.", Compute plan "0.5 CPU 1 GB RAM" $13.30/month, Storage "5 GB" $1.05/month (the new plan's floor), Total $14.35/month, "Your database will be unavailable for a few minutes while updating.", Update compute. No High availability or Read replicas line, as Render draws them only when HA is on or replicas exist. Cancel closed it with Basic 1GB still picked; the picker's own Cancel put Basic 256MB back | holds |
| Storage review (DoD 5) | Edit storage offered 5, Save: "Review your storage selection.", Compute plan "0.1 CPU 256 MB RAM" $4.20/month, Storage "5 GB" $1.05/month, Total $5.25/month, no warning, Update storage. Cancel returned to the size field holding 5 | holds |
| Key Value review (DoD 5) | Free → Starter, Save: "Review your compute selection.", Compute plan "0.1 CPU 256 MB RAM 250 connections" $7.00/month, Total $7.00/month, "Your Key Value instance will be unavailable while updating.", Upgrade compute (persistence at its default, so no data-loss sentence). Cancel closed it with Starter still picked | holds |
| Layout | each review inside the viewport (576 px wide at 1440, 358 px at 390, buttons stacked at 390), no sideways scroll, no page or console error | holds |
| Nothing applied | REST after the walk: the Postgres `basic_256mb` with `diskSizeGB` 1, the Key Value `free` | holds |

Both stores were deleted after the walk (REST `DELETE` 204, then 404) and the QA session revoked (`qa-login.sh --logout`, then REST 401). DoD 1–3 were walked with real changes locally (§ Local walk); production stops at Cancel, as DoD 5 says.

## Unblock record

- **2026-11-27, `/loopx w3` (verified evidence, no user decision):** the gate, pin must carry `2a8e5537f3`, holds. The newest pin on `origin/main` is `9c76d4ae72` `chore(deploy): pin platform images to 6ba91ca5b72f`, `git merge-base --is-ancestor 2a8e5537f3 6ba91ca5b72f` exits 0, and production's `dashboard` and `bex-system` Deployments run that pin's digests. Affects t005 (step 2) and t009. No gate remains.
- **2026-11-27, `/loopx w3`, closed:** t005 step 2 held definition of done item 5 on production pin `6ba91ca5b72f` (§ Production walk), with both stores deleted and the QA session revoked. Every point of the definition of done holds; t009 moved the milestone to `done/`.
