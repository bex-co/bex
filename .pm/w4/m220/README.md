# w4 · m220 — Keep list headers readable and loading previews stable

**Worker:** worker4 **Goal:** Environment Groups and Blueprints have readable mobile table headers, and the group and webhook lists reserve their ready header, action, search and table geometry while loading. **Status:** todo（2026-10-31 解除阻塞；已完成任务保持 DONE，详见文末“解除阻塞记录”）

**Estimate:** 3h40 (10 tasks). Implemented 2026-10-16; awaiting the deployed-build replay (t008).

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Keep list column labels on readable lines — **DONE** | 20m | — |
| t002 | Reserve the exact translated page header and action — **DONE** | 20m | — |
| t003 | Give the list and its previews one column layout — **DONE** | 25m | t001 |
| t009 | Keep Blueprint list column labels on readable lines — **DONE** | 15m | — |
| t010 | Match the webhook list pending card to its ready layout — **DONE** | 25m | — |
| t004 | Audit scoped loading callers and adjacent list states — **DONE** | 25m | t002, t003, t009, t010 |
| t005 | Check Render parity and unchanged API contracts — **DONE** | 15m | t004 |
| t006 | Simplify the changed code — **DONE** | 10m | t005 |
| t007 | Test real browser geometry and ready controls — **DONE** | 45m | t005 |
| t008 | Replay the live evidence and close out | 20m | t006, t007 |

## Definition of done

- Fresh-load populated `/env-groups` at390×844 and360×844, then horizontally scroll its table. **Env Vars** and **Secret Files** remain readable labels on one line; all five column headers share the ordinary36px desktop header height, rather than the captured200.5px row. Columns may scroll inside the existing table container; the document stays at the viewport width. Preserve row names, scope, counts and dates.
- Fresh-load `/blueprints` at390/360px as in [a822](finding-a822.md): all five short table labels remain on one line in a36px header, rather than180.5/220.5px, with horizontal overflow confined to its existing table scroller and document width matching viewport. Preserve the1440px36px control, data/links/repository/status/date semantics. No Blueprint pending-layout guarantee is inferred.
- Preserve the observed1440px control: all five labels are readable, ordinary header height stays36px and the content remains within its1024px maximum width.
- Repeat the exact native pending probe in [durable evidence](../evidence/m220-a821-env-groups-layout.json) at1440,390 and360px, releasing successful fetched responses unchanged. Pending and ready page-header bounds, translated title/description wrapping and New Environment Group action footprints agree at each width. The mobile search keeps its y position through the swap; currently it moves191→209px. The action reserves its ready202.17×32px footprint, rather than a96×36px block.
- The five pending table columns use the repaired ready table's minimum widths and horizontal scroll container. Do not compress the preview into five equal narrow mobile cells or reproduce the broken200px header. Representative data-dependent row counts remain allowed; matching the one-row fixture's exact height is not required.
- Repeat the exact [a832 V2 webhook-list response delay](../evidence/m220-a832-webhooks-loading.json) at390×844 and1440×844. The pending card preserves the ready title/translated description/action grid and single search/help region, with matching static bounds and table top. Mobile action/search placeholders fit the card rather than ending at x401 beyond its x374 edge. Preserve the existing ready layout and five-column inner scroller; representative row counts remain allowed. No webhook ready-header nowrap target or other webhook route is added.
- No resource writes are needed for replay. Inventory remains unchanged, this run's session is revoked, and dated fixed-build receipts support closure. Browser-local CSS from the hunt is a diagnostic control, not live completion evidence.

## Source + Goal linkage

- **Source:** ordinary functional `$qa-find-bugs w4`, pass a821, 2026-10-17 UTC; [two researched findings and prior-contract audit](finding.md), [exact scripts, requests/responses, DOM geometry and cleanup](../evidence/m220-a821-env-groups-layout.json). Research main `5448f8d9c3f30db207bc896b83985f4fdefbac42`.
- **Goal linkage:** ADR008's dependable Render alternative, ADR018's Environment Groups dashboard surface, and the supplied AGENTS.md requirement that pending geometry match ready content at the same breakpoint.
- **Expected outcome:** customers can read group counts on mobile and the list does not move its search or replace its column structure when loading finishes.
- **Why now:** repeated fresh360/390px loads produce vertical labels and a200px header. Three successful native-response pending replays show a distinct header/action/column mismatch. A local nowrap diagnostic restores36px headers without document overflow.
- **Scope:** two ready tables/ten header sites (Environment Groups and Blueprints), plus the canonical Environment Groups list, its one route-pending component and the two production EnvGroupsContentSkeleton mounts, and the canonical Webhooks list pending card. Preserve the global wrapping rule, Table primitives, the other two direct generic-page-header consumers enumerated in the original finding, and unrelated tables. Coordinate with m219's separate search-readiness gate without duplicating its state work.
- **Follow-up a822:** [Blueprint list header wrapping](finding-a822.md), with [six full DOM/control/cleanup receipts](../evidence/m220-a822-blueprint-headers.json). Add this second allowlisted ready table; generic TableHead and global wrapping stay unchanged. Nine tasks total2h55. Blueprint pending geometry and page-title/body-cell wrapping remain outside this observed target.
- **Follow-up a832:** [Webhook list pending geometry](finding-a832.md), with [three valid unchanged-response replays and complete safe native/cleanup receipts](../evidence/m220-a832-webhooks-loading.json). Add one allowlisted pending card; preserve TableCardSkeleton and its other nine direct JSX sites. Final scope: ten tasks,3h40. This is an uncovered portion of w5/m79/t007, not the distinct Settings issue295 or a third ready-header fix.
- **Render parity:** included because the visible dashboard changes. Render's public docs establish list/group management, not this exact layout or pending timing. No authenticated Render visual comparison was run; preserve API fields, error handling and data.
- **Dedupe:** reviewed82 actual open/blocked milestone titles, open/done/blocked text searches, DO_NOT_DO, current source and recent/targeted history. The pending defect is a gap in completed w5/m79's geometry contract, audited item by item in the finding. It does not claim the whole82-route guarantee was reverified. Existing m219 input readiness,304 scope labels,305 cron layout and298 datastore cards have different owners/mechanisms.

## Implementation record (2026-10-16)

- **t001/t009 — readable ready headers.** `EnvGroupsTableHeader` (new, `features/env-groups/components/env-groups-table-header.tsx`) holds the group list's five `whitespace-nowrap` labels (counts and Updated right-aligned) and is shared by the ready list and its preview. The five Blueprint list `TableHead`s get `whitespace-nowrap`. Shared `Table`/`TableHead` primitives and the global `overflow-wrap: anywhere` rule are unchanged; overflow stays in the existing table scroller.
- **t002 — group page header.** `EnvGroupsPageHeaderSkeleton` reserves the translated title and description (`ReservedText`) and the New Environment Group trigger's exact `size="sm"` Plus + label footprint (`ReservedButton`).
- **t003 — one column layout.** `EnvGroupsContentSkeleton` (both mounts) renders a real `Table` with `EnvGroupsTableHeader` and three rows using the ready cells' `min-w-48`/`min-w-40`, right-aligned counts and nowrap date, replacing the five equal grid cells.
- **t010 — webhook list card.** `WebhooksListPageSkeleton` now renders `WebhooksCardPreview` (CardTitle/CardDescription/CardAction slot grid with the translated copy and the `outline sm` Plus "Create" action, one `max-w-md` search + help region in CardContent, then the ready table's columns via a new shared `WebhooksTableHeader` that the ready panel also renders). `TableCardSkeleton` and its other nine sites are unchanged. Found and fixed while measuring: reserved copy wrapped in a `<p>` gets its `<div>` bar re-parented by the HTML parser, so the pending header and `ReservedHelp` (now sized by an optional class) use positioned `<div>`s (the 295 Settings probe recognises that slot).
- **t004 — audit.** Callers: one route-pending component per list, two `EnvGroupsContentSkeleton` mounts, `PageHeaderSkeleton` still used by Blueprints; the panel's own `PanelTableSkeleton` branch, empty/no-match/error states and the m219 search gate are unchanged.
- **t005 — parity/contracts.** Layout only: no GraphQL/REST/MCP field, permission, error or data change. Render's docs specify no authenticated loading layout; no Render visual comparison was made.
- **t007 — browser geometry** (production CSS compiled by `vite build`, real captured DOM, Chromium). Group and Blueprint headers 200.5/220.5px → 36px at 390/360 and 36px at 1440, document width = viewport. Group pending = ready: header 145/81px, search y 161/105px, action 202.17×32px, both tables 1022px at 1440. Webhooks pending = ready: header 160/80px, content top 197/117px, table top 271/191px, action 133.95×32px, zero overhang at 390/1440. Tests: `routes/__tests__/env-groups.test.tsx` and `blueprints.test.tsx` (nowrap labels, preview labels = ready labels), `webhooks-panel.test.tsx` (pending title/action/columns = ready); each fails on the old code. Dashboard suite, lint and typecheck pass.
- **Unverified:** 360px webhooks, Chinese layouts, other roles, reduced motion; the live a821/a822/a832 replays are t008.

## 解除阻塞记录（2026-10-31，`/unblock w4`）

- **涉及：** w4/m220（同组 D2：w4/m237、w4/m219、w4/m212、w4/m172、w4/295）
- 来源：用户明确决定（2026-10-31，`/unblock w4`）——“i approve you to update bex-canary's subscription to paid and operate anything there but please clean up afterwards”；以及 2026-10-30：“qa-* 的数据库都是暂时的可以清除”。
- **结论（D2，Pro+ 门槛）：** 允许 worker 用 `ChangeWorkspacePlan` 把 bex-canary 切到 Pro（或该功能所需最低付费档），在一个窗口内集中重放全部 Pro 门槛项（w4/m237、m219、m220、m212、m172、295），删除自建 webhook 等资源后切回 Hobby，并在各项记录升/降级时间。窗口期内 bex-canary 读作 Pro，其他会话的 Hobby 限制类测试读数可能不同，因此窗口尽量短。不使用 `bex`（Scale，真实计费）workspace。 bex-canary（`tea-daif693dqjvc73e7as3g`）计费状态为 `operator_excluded: structural exclusion`，不走 Stripe 收款（2026-10-29 只读核实 `workspaceBillingReadiness`），付费 fixture 与付费套餐不产生真实费用。
- 生产已部署 `25b99d8a373a`（`deploy/gitops/base/bex.yaml`，2026-10-30 pin），本项实现均早于此版本；部署不等于验收，以下仍是待验证工作。
- **worker 下一步：** 与同组其他项排在同一个 Pro 窗口内，按本项原 DoD 重放剩余的 Pro 门槛检查。
- **清理（用户要求，验收的一部分）：** 验收结束删除本次在 bex-canary 创建的全部资源；若为此把 bex-canary 切到付费套餐，所有依赖付费档的项做完后切回 Hobby；在本条记录清理结果（删除后 404 / 列表为空、套餐读回 Hobby）。
- **剩余阻塞：** 无（门槛已核实解除，交给 `/loopx w4`）。
- **验收：** 原 DoD / Acceptance 不变，未删减。
