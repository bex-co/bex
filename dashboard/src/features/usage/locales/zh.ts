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
  "usage.resourceCapsNearLimit": {
    message: "接近上限",
    description: "Warning shown once a resource count reaches 80 percent",
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
    message: "其中 ${amount} 将于 {date} 过期",
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
    message: "已抵扣 \u2212${applied} \u2192 应付 ${due}",
    description:
      "Credit applied to the current period and the remaining amount due",
  },
  "usage.chargesDescriptionPending": {
    message: "本期累计用量，按 bex 价目表计价。",
    description:
      "Charges card description while the invoiced total is still loading; deliberately states only what is already true, without claiming the figure is or is not a Stripe invoice.",
  },
};

export default zhUsage;
