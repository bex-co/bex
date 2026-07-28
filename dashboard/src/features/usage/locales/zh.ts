import type { TranslationEntry } from "@/i18n";

const zhUsage: Record<string, TranslationEntry> = {
  "usage.pageTitle": {
    message: "用量",
    description: "Usage page heading and browser title",
  },
  "usage.pageSubtitle": {
    message: "工作区本月累计消耗",
    description: "Usage page subtitle beneath the heading",
  },
  "usage.resourceCapsTitle": {
    message: "资源限制",
    description: "Workspace creation-cap card title on the Usage page",
  },
  "usage.resourceCapsDescription": {
    message: "当前工作区的资源数量",
    description: "Workspace creation-cap card description",
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
  "usage.resourceCapsValue": {
    message: "已使用 {used}/{limit}",
    description: "Used-versus-limit resource count",
  },
  "usage.resourceCapsNearLimit": {
    message: "接近上限",
    description: "Warning shown once a resource count reaches 80 percent",
  },
  "usage.computeTitle": {
    message: "计算",
    description: "Compute section heading on the Usage page",
  },
  "usage.computeDescription": {
    message: "按服务和套餐统计的实例小时数",
    description: "Compute section description",
  },
  "usage.bandwidthTitle": {
    message: "带宽",
    description: "Bandwidth section heading on the Usage page",
  },
  "usage.bandwidthDescription": {
    message: "HTTP、WebSocket、直连公网及公共数据存储响应的出站流量",
    description: "Bandwidth section description",
  },
  "usage.buildTitle": {
    message: "构建分钟数",
    description: "Build minutes section heading on the Usage page",
  },
  "usage.buildDescription": {
    message: "构建消耗的流水线分钟数",
    description: "Build section description",
  },
  "usage.storageTitle": {
    message: "存储",
    description: "Managed datastore storage section heading on the Usage page",
  },
  "usage.storageDescription": {
    message: "Postgres 与 Key Value 卷的实际用量",
    description: "Storage section description",
  },
  "usage.colService": {
    message: "服务",
    description: "Table column header: service name",
  },
  "usage.colKind": {
    message: "类型",
    description:
      "Table column header: resource kind (service/postgres/key_value)",
  },
  "usage.colPlan": {
    message: "套餐",
    description: "Table column header: service plan/tier",
  },
  "usage.colHours": {
    message: "小时数",
    description: "Table column header: compute hours",
  },
  "usage.colBandwidth": {
    message: "带宽",
    description: "Table column header: egress bandwidth",
  },
  "usage.colMinutes": {
    message: "分钟数",
    description: "Table column header: build minutes",
  },
  "usage.colGBHours": {
    message: "GB 小时",
    description: "Table column header: storage gigabyte-hours",
  },
  "usage.totalRow": {
    message: "合计",
    description: "Summary row label in usage tables",
  },
  "usage.empty": {
    message: "本月暂无用量记录。",
    description: "Empty-state message when a section has no data",
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
  "usage.trendTitle": {
    message: "近三月趋势",
    description: "Heading for the trend view showing last 3 months of usage",
  },
  "usage.trendDescription": {
    message: "近三个自然月的各计量指标合计",
    description: "Subtitle for the trend charts on the Usage page",
  },
  "usage.estimatedCostTitle": {
    message: "预估费用",
    description: "Estimated cost section heading on the Usage page",
  },
  "usage.estimatedCostDescription": {
    message:
      "计算、Postgres、Key Value 及 Postgres 存储比 Render 低 30%；带宽低 90%。仅供参考，非正式账单。",
    description:
      "Estimated cost section description explaining the pricing policy",
  },
  "usage.estimatedCostNote": {
    message: "仅供参考，非正式账单",
    description: "Short disclaimer shown next to the estimated cost total",
  },
  "usage.colMeter": {
    message: "计量项",
    description: "Table column header for the usage meter kind",
  },
  "usage.colEstimate": {
    message: "预估",
    description: "Table column header for the estimated USD cost per meter",
  },
  "usage.estimatedCostUnavailable": {
    message: "本期无计费用量。",
    description:
      "Empty-state message when there is no billable usage to estimate",
  },
  "usage.currentSpendTitle": {
    message: "当前消费",
    description:
      "Heading for the real billing section (actual Stripe cost + invoices)",
  },
  "usage.currentSpendBadge": {
    message: "账单",
    description:
      "Badge distinguishing the real billing card from the estimate-only card",
  },
  "usage.currentSpendDescription": {
    message: "您的实际计费金额与已出具的账单——并非估算。",
    description: "Description clarifying this card shows real charges",
  },
  "usage.currentSpendNote": {
    message: "当前计费周期，截至目前",
    description: "Short note beside the current-period real cost total",
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
  "usage.billingTaxUnconfigured": {
    message:
      "税务尚未配置。在运营人员确认规范的产品税码和有效的测试注册前，税费收取会保持关闭。",
    description: "Fail-closed tax setup explanation",
  },
  "usage.billingAddPayment": {
    message: "添加测试付款方式",
    description: "Button opening setup-mode Stripe Checkout",
  },
  "usage.billingUpdatePayment": {
    message: "更新测试付款方式",
    description: "Button reopening setup-mode Stripe Checkout",
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
    message: "账单设置暂不可用，或您没有工作区管理员权限。",
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
};

export default zhUsage;
