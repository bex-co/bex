import type { TranslationEntry } from "@/i18n";

const zhMetrics: Record<string, TranslationEntry> = {
  "metrics.subtitle": {
    message: "指标——来自 bex-api 的实时数据",
    description: "Subtitle on the service metrics page",
  },
  "metrics.networkTitle": {
    message: "网络指标",
    description: "Network metrics card title",
  },
  "metrics.networkDescription": {
    message: "汇总所有实例的数据",
    description: "Network metrics card description",
  },
  "metrics.totalRequests": {
    message: "请求总数",
    description: "Network metrics chart section title",
  },
  "metrics.responseTimes": {
    message: "响应时间（{quantile}）",
    description: "Network metrics chart section title, with the selected quantile",
  },
  "metrics.outboundBandwidth": {
    message: "出站带宽",
    description: "Network metrics chart section title",
  },
  "metrics.sourceNotConfigured": {
    message: "未配置指标数据源",
    description: "Shown when bex-api reports no metrics backend is wired up",
  },
  "metrics.applicationTitle": {
    message: "应用指标",
    description: "Application metrics card title",
  },
  "metrics.applicationDescription": {
    message: "来自 metrics-server 的单实例资源使用情况",
    description: "Application metrics card description",
  },
  "metrics.memory": {
    message: "内存",
    description: "Application metrics stat tile label",
  },
  "metrics.cpu": {
    message: "CPU",
    description: "Application metrics stat tile label",
  },
  "metrics.totalInstances": {
    message: "实例总数",
    description: "Application metrics stat tile label",
  },
  "metrics.filterPercentage": {
    message: "百分比",
    description: "Metrics filter tab: show values as a percentage",
  },
  "metrics.filterTotal": {
    message: "总量",
    description: "Metrics filter tab: show values as an absolute total",
  },
  "metrics.rangeLast30Minutes": {
    message: "过去 30 分钟",
    description: "Metrics time-range filter option",
  },
  "metrics.rangeLastHour": {
    message: "过去一小时",
    description: "Metrics time-range filter option",
  },
  "metrics.rangeLast12Hours": {
    message: "过去 12 小时",
    description: "Metrics time-range filter option",
  },
};

export default zhMetrics;
