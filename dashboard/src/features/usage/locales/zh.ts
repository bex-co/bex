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
};

export default zhUsage;
