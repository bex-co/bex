import type { TranslationEntry } from "@/i18n";

const zhUsage: Record<string, TranslationEntry> = {
  "usage.pageTitle": {
    message: "账单",
    description:
      "Billing page heading and browser title (renamed from Usage in w5/m70)",
  },
  "usage.pageSubtitle": {
    message: "付款、发票与工作区本月累计消耗",
    description: "Billing page subtitle beneath the heading",
  },
  "usage.sectionNavigation": {
    message: "账单区块",
    description: "Accessible label for the billing page section navigation",
  },
  "usage.resourceCapsServices": {
    message: "服务",
    description: "Service count label in the resource-cap card",
  },
  "usage.resourceCapsPostgres": {
    message: "Postgres",
    description: "Postgres count label in the resource-cap card",
  },
  "usage.resourceCapsKeyValues": {
    message: "Key Value",
    description: "Key Value count label in the resource-cap card",
  },
  "usage.resourceCapsFree": {
    message: "免费：{used} / {limit}",
    description: "Free-instance count in the resource-cap card (w4/m241)",
  },
  "usage.freeHoursTitle": {
    message: "每月包含用量",
    description:
      "Title of the Free instance hours card on the usage page (w8/m139)",
  },
  "usage.freeHoursDescription": {
    message:
      "每个工作区的免费 Web 服务共享每月的实例小时额度。休眠时间不计入。",
    description: "Explains the monthly Free instance hours allowance (w8/m139)",
  },
  "usage.freeHoursUsed": {
    message: "本月已使用 {used} / {included} 免费实例小时",
    description:
      "Free instance hours used against the monthly allowance (w8/m139)",
  },
  "usage.freeHoursNear": {
    message: "额度即将用尽。用尽后，您的免费 Web 服务将暂停，直到下月初。",
    description:
      "Warning once Free instance hours reach 80 percent of the allowance (w8/m139)",
  },
  "usage.freeHoursSpent": {
    message:
      "额度已用尽。免费 Web 服务暂停至下月初；将服务升级到付费套餐即可立即运行。",
    description:
      "Notice once the monthly Free instance hours are spent (w8/m139)",
  },
  "usage.buildMinutesTitle": {
    message: "流水线分钟",
    description:
      "Title of the included build minutes card on the billing page (w6/m308)",
  },
  "usage.buildMinutesDescription": {
    message:
      "构建和部署前命令会消耗流水线分钟。您的套餐每月包含一定额度；超出部分按分钟计费。",
    description: "Explains the plan's included build minutes (w6/m308)",
  },
  "usage.buildMinutesUsed": {
    message: "本月已使用 {used} / {included} 包含的构建分钟",
    description:
      "Build minutes used against the plan's included minutes (w6/m308)",
  },
  "usage.buildMinutesNear": {
    message:
      "包含的构建分钟即将用尽。如果没有付款方式，用尽后新的构建和部署前命令将停止，直到下月初。",
    description:
      "Warning once build minutes reach 80 percent of the included minutes (w6/m308)",
  },
  "usage.buildMinutesSpent": {
    message:
      "包含的构建分钟已用尽。超出部分按分钟计费；如果没有付款方式，新的构建和部署前命令将停止，直到下月初。",
    description: "Notice once the included build minutes are spent (w6/m308)",
  },
  "usage.bandwidthTitle": {
    message: "出站流量",
    description:
      "Title of the included bandwidth card on the billing page (w6/m319)",
  },
  "usage.bandwidthDescription": {
    message:
      "服务向互联网发送的流量会消耗带宽。您的套餐每月包含一定额度；超出部分按 GB 计费。",
    description: "Explains the plan's included outbound bandwidth (w6/m319)",
  },
  "usage.bandwidthUsed": {
    message: "本月已使用 {used} / {included} GB 包含的流量",
    description:
      "Outbound bandwidth used against the plan's included bandwidth, in GB (w6/m319)",
  },
  "usage.bandwidthNear": {
    message:
      "包含的流量即将用尽。如果没有付款方式，用尽后工作区的服务将停止运行，直到下月初。",
    description:
      "Warning once bandwidth reaches 80 percent of the included bandwidth (w6/m319)",
  },
  "usage.bandwidthSpent": {
    message:
      "包含的流量已用尽。超出部分按 GB 计费；如果没有付款方式，工作区的服务将停止运行，直到下月初。",
    description: "Notice once the included bandwidth is spent (w6/m319)",
  },
  "usage.chargeIncluded": {
    message: "其中 {included} 为套餐包含",
    description:
      "Part of a charge line's quantity that the plan's included build minutes or bandwidth covered (w6/m308, w6/m319)",
  },
  "usage.pipelineTitle": {
    message: "构建流水线",
    description:
      "Title of the Build Pipeline card in workspace settings (w6/m309)",
  },
  "usage.pipelineDescription": {
    message:
      "构建和部署前命令会消耗流水线分钟。超出套餐包含额度的分钟按分钟计费，不超过您设置的每月消费上限。达到上限后，构建将停止到下月。",
    description: "Explains the pipeline spend limit (w6/m309)",
  },
  "usage.pipelineSpendLimitValue": {
    message: "消费上限：每月 {limit}",
    description: "The workspace pipeline spend limit (w6/m309)",
  },
  "usage.pipelineSpendLimitNone": {
    message: "未设置消费上限",
    description: "The workspace has no pipeline spend limit (w6/m309)",
  },
  "usage.pipelineSpendLimitSet": {
    message: "设置消费上限",
    description: "Button that opens the pipeline spend limit dialog (w6/m309)",
  },
  "usage.pipelineSpendLimitEdit": {
    message: "编辑",
    description: "Button that edits the pipeline spend limit (w6/m309)",
  },
  "usage.pipelineSpendLimitRemove": {
    message: "移除上限",
    description: "Button that removes the pipeline spend limit (w6/m309)",
  },
  "usage.pipelineSpendLimitDialogTitle": {
    message: "设置消费上限",
    description: "Title of the pipeline spend limit dialog (w6/m309)",
  },
  "usage.pipelineSpendLimitDialogDescription": {
    message:
      "此工作区每月在超出套餐包含额度的构建分钟上的最高花费。达到后，新的构建和部署前命令将停止，直到下月初。",
    description: "Explains what the pipeline spend limit caps (w6/m309)",
  },
  "usage.pipelineSpendLimitLabel": {
    message: "每月上限（美元）",
    description: "Label of the pipeline spend limit input (w6/m309)",
  },
  "usage.pipelineSpendLimitSave": {
    message: "保存",
    description: "Saves the pipeline spend limit (w6/m309)",
  },
  "usage.pipelineSpendLimitSaved": {
    message: "消费上限已设为每月 {limit}",
    description: "Toast after setting the pipeline spend limit (w6/m309)",
  },
  "usage.pipelineSpendLimitRemoved": {
    message: "已移除消费上限",
    description: "Toast after removing the pipeline spend limit (w6/m309)",
  },
  "usage.pipelineSpendLimitError": {
    message: "无法更新消费上限",
    description: "Toast when updating the pipeline spend limit fails (w6/m309)",
  },
  "usage.pipelineSpendLimitUnreadable": {
    message: "无法加载消费上限。请重新加载后重试。",
    description: "The pipeline spend limit could not be read (w6/m309)",
  },
  "usage.pipelineSpendLimitForbidden": {
    message: "只有账单管理员可以查看和更改消费上限。",
    description: "The caller may not read the pipeline spend limit (w6/m309)",
  },
  "usage.resourceCapsNearLimit": {
    message: "接近上限",
    description:
      "Warning shown once a resource count reaches 80 percent of its cap, below the cap itself",
  },
  "usage.resourceCapsLimitReached": {
    message: "已达上限",
    description:
      "Shown when a resource count is at or over its cap, so creating another is refused",
  },
  "usage.resourceCapsFinishingDeletion": {
    message: "{count} 个正在删除",
    description:
      "Sub-line on a resource-cap tile: how many of the counted resources are mid-deletion and thus hidden from the resource list but still holding quota (w6/m129)",
  },
  "usage.resourceCapsFinishingDeletionHint": {
    message: "删除完成前仍占用配额",
    description:
      "Tooltip explaining why resources finishing deletion are included in the used count",
  },
  "usage.errorTitle": {
    message: "无法加载用量数据",
    description: "Error state heading on the Usage page",
  },
  "usage.monthPickerLabel": {
    message: "选择月份",
    description: "Aria-label for the month-picker select on the Usage page",
  },
  "usage.currentMonth": {
    message: "当月",
    description:
      "Default option in the month picker meaning the current calendar month",
  },
  "usage.creditsTitle": {
    message: "剩余额度",
    description: "Heading of the remaining billing-credit card (w5/m70)",
  },
  "usage.creditsDescription": {
    message: "促销额度会在扣款前先抵扣发票金额",
    description: "Subtitle of the remaining billing-credit card",
  },
  "usage.creditsExpiryNote": {
    message: "其中 {amount} 将于 {date} 过期",
    description:
      "Earliest-expiring grant note beside the credit balance; amount is a $ value, date is YYYY-MM-DD",
  },
  "usage.creditsCardStillRequired": {
    message:
      "即使持有额度也仍需绑定付款方式：额度优先抵扣发票，剩余部分由银行卡支付。",
    description:
      "ADR046 clarification on the credit card — credit does not replace payment onboarding",
  },
  "usage.colInvoicePeriod": {
    message: "周期",
    description: "Table column header for an invoice's billing period start",
  },
  "usage.colInvoiceStatus": {
    message: "状态",
    description: "Table column header for an invoice's status",
  },
  "usage.colInvoiceAmount": {
    message: "金额",
    description: "Table column header for an invoice's amount",
  },
  "usage.billingSetupTitle": {
    message: "账单设置",
    description: "Customer billing onboarding card title",
  },
  "usage.billingSetupDescription": {
    message: "此工作区的付款收取与税务就绪状态",
    description: "Customer billing onboarding card description",
  },
  "usage.billingTestMode": {
    message: "Stripe 测试模式",
    description: "Badge making the non-live Stripe environment explicit",
  },
  "usage.billingMode": {
    message: "Stripe Billing",
    description: "Fallback badge for the Stripe billing environment",
  },
  "usage.billingReady": {
    message: "已就绪",
    description: "Billing onboarding status is ready",
  },
  "usage.billingActionNeeded": {
    message: "需要操作",
    description: "Billing onboarding status needs action",
  },
  "usage.billingCustomerStatus": {
    message: "客户",
    description: "Stripe Customer readiness row",
  },
  "usage.billingSubscriptionStatus": {
    message: "计量订阅",
    description: "Stripe Subscription readiness row",
  },
  "usage.billingPaymentStatus": {
    message: "付款方式",
    description: "Default payment method readiness row",
  },
  "usage.billingTaxStatus": {
    message: "自动计税",
    description: "Stripe Tax activation row",
  },
  "usage.billingLifecycleStatus": {
    message: "收款生命周期",
    description: "Stripe collection and reversible enforcement state row",
  },
  "usage.billingLifecycleGrace": {
    message:
      "付款失败（{reason}）。宽限期内工作区仍可用；可逆暂停将在 {deadline} 后执行。",
    description: "Visible billing grace state and authoritative deadline",
  },
  "usage.billingLifecycleEnforced": {
    message:
      "账单限制已生效（{reason}）。计算资源已暂停，但数据库、Key Value 数据、Secrets 与账单证据均未删除。",
    description: "Visible reversible billing enforcement state",
  },
  "usage.billingLifecycleRecovering": {
    message: "付款已恢复。bex 正在仅恢复由账单限制所变更的资源。",
    description: "Visible precise recovery state",
  },
  "usage.billingLifecycleExcluded": {
    message: "运营人员已将此工作区排除在 Stripe 收款之外。",
    description: "Visible structural billing exclusion state",
  },
  "usage.billingLifecycleComped": {
    message: "运营人员已为此工作区应用全额减免，但仍保留计价记录。",
    description: "Visible rated-but-free comp state",
  },
  "usage.billingLifecycleUnknown": {
    message: "账单状态为 {reason}。请使用账单门户或联系支持。",
    description: "Forward-compatible unknown billing lifecycle state",
  },
  "usage.billingNoDeadline": {
    message: "未报告截止时间",
    description: "Fallback when a malformed grace state has no deadline",
  },
  "usage.billingOff": {
    message: "未启用",
    description:
      "Neutral status for a deliberately disabled billing capability (e.g. tax not activated)",
  },
  "usage.billingAddPayment": {
    message: "添加付款方式",
    description: "Button opening setup-mode Stripe Checkout (live mode)",
  },
  "usage.billingUpdatePayment": {
    message: "更新付款方式",
    description: "Button reopening setup-mode Stripe Checkout (live mode)",
  },
  "usage.billingAddPaymentTest": {
    message: "添加测试付款方式",
    description:
      "Button opening setup-mode Stripe Checkout when Stripe is in test mode",
  },
  "usage.billingUpdatePaymentTest": {
    message: "更新测试付款方式",
    description:
      "Button reopening setup-mode Stripe Checkout when Stripe is in test mode",
  },
  "usage.billingOpenPortal": {
    message: "打开账单门户",
    description: "Button opening Stripe Customer Portal",
  },
  "usage.billingHostedNote": {
    message:
      "付款信息只会在 Stripe 托管页面中输入；bex 不会向浏览器发送 Stripe 服务端密钥。",
    description: "Hosted billing security note",
  },
  "usage.billingUnavailable": {
    message: "账单设置暂不可用，或您没有账单权限（账单角色或管理员）。",
    description: "Degraded or unauthorized billing onboarding state",
  },
  "usage.billingCheckoutError": {
    message: "无法打开 Stripe Checkout，请重试。",
    description: "Toast after Checkout session creation fails",
  },
  "usage.billingPortalError": {
    message: "无法打开 Stripe 账单门户，请重试。",
    description: "Toast after Portal session creation fails",
  },
  "usage.paymentRequiredTitle": {
    message: "添加付款方式以继续",
    description: "Just-in-time paid-intent onboarding dialog title",
  },
  "usage.paymentRequiredDescription": {
    message:
      "此付费方案需要付款方式。请在新标签页中完成 Stripe Checkout；准备就绪后，此操作将自动继续。",
    description: "Just-in-time paid-intent onboarding explanation",
  },
  "usage.paymentRequiredRetrying": {
    message: "已找到付款方式，正在重试操作…",
    description: "Status while resuming the interrupted paid mutation",
  },
  "usage.paymentRequiredCancel": {
    message: "取消",
    description: "Cancel just-in-time payment onboarding",
  },
  "usage.planTitle": {
    message: "计划",
    description: "Plan card title on the billing page",
  },
  "usage.planChange": {
    message: "更改计划",
    description:
      "Link from the billing page's plan card to the workspace-settings plan dialog",
  },
  "usage.paymentMethodTitle": {
    message: "付款方式",
    description: "Payment-method card title on the billing page",
  },
  "usage.paymentMethodCard": {
    message: "{brand} 尾号 {last4}",
    description: "Names the card on file, e.g. 'Visa ending in 4242'",
  },
  "usage.paymentMethodOnFile": {
    message: "已保存付款方式",
    description:
      "Shown when a payment method exists but is not a card the provider could name",
  },
  "usage.paymentMethodNone": {
    message: "未添加付款方式",
    description: "Shown when the workspace has no payment method",
  },
  "usage.includedUsageTitle": {
    message: "包含用量",
    description:
      "Plan-allowance card title on the billing page (was 'Resource limits')",
  },
  "usage.includedUsageDescription": {
    message: "该工作区计划包含的额度，以及已使用的部分。",
    description: "Included-usage card description",
  },
  "usage.chargesTitle": {
    message: "费用",
    description: "Charge-tree card title on the billing page",
  },
  "usage.chargesDescriptionEstimate": {
    message: "本周期至今的累计费用，按 bex 价目表计算。仅为估算，非发票。",
    description:
      "Charge-tree description when no Stripe subscription prices the period",
  },
  "usage.chargesDescriptionInvoiced": {
    message: "本周期至今的累计费用，由 Stripe 计价。",
    description:
      "Charge-tree description when a real Stripe amount is available. Says the total is Stripe's rating rather than the amount it will invoice: credits and comp discounts can sit between the two, and the amount actually due gets its own line (w6/m98).",
  },
  "usage.amountDueAfterCredits": {
    message: "抵扣后应付",
    description:
      "Label for the charge-tree line showing what Stripe actually collects once credits and discounts are applied to the charge above it",
  },
  "usage.chargesEmpty": {
    message: "本周期没有用量。",
    description: "Charge-tree empty state",
  },
  "usage.coveragePartial": {
    message: "数据不完整",
    description:
      "Amber caveat label above the charges when the metering behind the estimate is degraded/incomplete (w4/048), mirroring the Metrics page's degraded badge",
  },
  "usage.coveragePartialLead": {
    message: "此估算可能低估了用量。",
    description: "Lead sentence of the partial-coverage caveat tooltip",
  },
  "usage.coveragePartialThrough": {
    message: "仅统计至 {through}。",
    description:
      "Partial-coverage caveat clause naming the date the estimate is complete through; {through} is a YYYY-MM-DD date",
  },
  "usage.coveragePartialSources": {
    message: "降级的计量来源：{sources}。",
    description:
      "Partial-coverage caveat clause listing the degraded metering source names; {sources} is a comma-separated list",
  },
  "usage.expandAll": {
    message: "全部展开",
    description: "Charge-tree button that opens every category",
  },
  "usage.collapseAll": {
    message: "全部收起",
    description: "Charge-tree button that closes every category",
  },
  "usage.totalToDate": {
    message: "本月至今合计",
    description: "Charge-tree total row for the month in progress",
  },
  "usage.totalForPeriod": {
    message: "本周期合计",
    description: "Charge-tree total row for a completed past month",
  },
  "usage.totalBillingPeriodWindow": {
    message: "本计费周期合计（{start} – {end}）",
    description:
      "Charge total row when the headline is Stripe's rated amount: it covers the subscription period (anchored on the subscription's day of month), not the calendar month (w4/m139)",
  },
  "usage.totalBillingPeriod": {
    message: "本计费周期合计",
    description:
      "Charge total row for Stripe's rated amount when its period bounds are unknown (w4/m139)",
  },
  "usage.breakdownMonthToDate": {
    message: "上方明细：{month}至今",
    description:
      "Row under a Stripe-rated total: the charge tree's own sum and window (the calendar month), which differs from the subscription period (w4/m139)",
  },
  "usage.projectedTotal": {
    message: "{month} 预计合计",
    description:
      "Charge-tree straight-line projection of month-end spend, for the estimate fallback whose total accrues over the calendar month",
  },
  "usage.projectedTotalBillingPeriod": {
    message: "本计费周期预计合计",
    description:
      "Charge-tree straight-line projection of a Stripe-rated total. The subscription period need not align with the calendar month (it can span two), so no month is named (w6/050).",
  },
  "usage.chargeFree": {
    message: "已包含",
    description: "Rate column value for a charge line that is priced at zero",
  },
  "usage.resourceDeleted": {
    message: "（已删除）",
    description:
      "Marker beside a charged resource that no longer exists; the name shown is the one bex retained",
  },
  "usage.categoryServices": {
    message: "服务",
    description: "Charge-tree category for App services",
  },
  "usage.categoryPostgres": {
    message: "Postgres",
    description: "Charge-tree category for managed Postgres instances",
  },
  "usage.categoryKeyValue": {
    message: "键值存储",
    description: "Charge-tree category for managed Key Value instances",
  },
  "usage.categoryAgentSessions": {
    message: "Agent 会话",
    description:
      "Charge-tree category for platform model usage of agent sessions",
  },
  "usage.chargeDisk": {
    message: "磁盘",
    description: "Charge line for a service's provisioned persistent disk",
  },
  "usage.chargeAgentModelCost": {
    message: "模型用量",
    description:
      "Charge line for platform model spend, billed at provider cost plus markup (ADR091)",
  },
  "usage.chargeAgentModelRate": {
    message: "服务商成本 + 30%",
    description: "Rate column value for the platform model spend line",
  },
  "usage.chargeAgentModelInputTokens": {
    message: "输入 token",
    description: "Breakdown line for platform model input tokens",
  },
  "usage.chargeAgentModelOutputTokens": {
    message: "输出 token",
    description: "Breakdown line for platform model output tokens",
  },
  "usage.chargeAgentModelCacheReadTokens": {
    message: "缓存读取 token",
    description: "Breakdown line for platform model cache-read tokens",
  },
  "usage.chargeAgentModelCacheWriteTokens": {
    message: "缓存写入 token",
    description: "Breakdown line for platform model cache-write tokens",
  },
  "usage.categorySandboxes": {
    message: "沙箱",
    description: "Charge-tree category for hosted agent sandboxes",
  },
  "usage.chargeCompute": {
    message: "计算",
    description: "Charge line for metered instance time",
  },
  "usage.chargeBandwidth": {
    message: "带宽",
    description: "Charge line for metered outbound bandwidth",
  },
  "usage.chargeBuild": {
    message: "构建时长",
    description: "Charge line for metered build time",
  },
  "usage.chargeStorage": {
    message: "存储",
    description: "Charge line for metered datastore storage",
  },
  "usage.chargeSandboxCompute": {
    message: "沙箱计算",
    description: "Charge line for metered sandbox compute",
  },
  "usage.creditsTotalLabel": {
    message: "余额合计",
    description: "Label above the credit balance amount",
  },
  "usage.invoiceHistoryTitle": {
    message: "发票记录",
    description: "Invoice-history card title on the billing page",
  },
  "usage.invoiceHistoryDescription": {
    message: "已开具的发票。",
    description: "Invoice-history card description",
  },
  "usage.creditsAppliedLine": {
    message: "已抵扣 −{applied} → 应付 {due}",
    description:
      "Credit applied to the current period and the remaining amount due",
  },
  "usage.chargesDescriptionPending": {
    message: "本期累计用量，按 bex 价目表计价。",
    description:
      "Charges card description while the invoiced total is still loading; deliberately states only what is already true, without claiming the figure is or is not a Stripe invoice.",
  },
  "usage.paymentMethodRejectedPrepaid": {
    message: "不接受预付卡；请添加信用卡或借记卡。",
    description:
      "Shown when the last card offered was refused at binding because it is prepaid (ADR091 D10.5)",
  },
  "usage.paymentMethodRejectedUnpaidBalance": {
    message: "此卡关联了一笔未付余额；请添加另一张卡。",
    description:
      "Shown when the last card offered was refused because it carries an unpaid written-off balance (ADR091 D10.6)",
  },
  "usage.agentModelTitle": {
    message: "智能体模型",
    description:
      "Billing page card and nav entry for agent model spend (ADR091 D6)",
  },
  "usage.agentModelPricing": {
    message: "按模型提供方成本 + 30% 计费。",
    description: "Pricing statement under the Agent model title",
  },
  "usage.agentModelMonthToDate": {
    message: "本月至今",
    description: "Label of the current month's agent model total",
  },
  "usage.agentModelPeriodTotal": {
    message: "当月合计",
    description: "Label of a closed month's agent model total",
  },
  "usage.agentModelBreakdown_other": {
    message: "按模型细分（{count} 个模型）",
    description: "Toggle that expands the per-model breakdown",
  },
  "usage.agentModelEmpty": {
    message: "此期间没有智能体模型用量。",
    description: "Shown when the period has no agent model spend",
  },
  "usage.lessThanAmount": {
    message: "不足 {amount}",
    description:
      "A charge that rounds below the smallest amount shown, e.g. <$0.01",
  },
};

export default zhUsage;
