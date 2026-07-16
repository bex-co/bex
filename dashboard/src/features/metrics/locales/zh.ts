import type { TranslationEntry } from "@/i18n";

const zhMetrics: Record<string, TranslationEntry> = {
  "metrics.eventsTitle": {
    message: "事件时间线",
    description:
      "Metrics-page timeline of service events in the selected range",
  },
  "metrics.eventsFilterLabel": {
    message: "筛选事件",
    description: "Accessible label for the Metrics event category filter",
  },
  "metrics.eventsFilterAll": {
    message: "全部事件",
    description: "All event categories",
  },
  "metrics.eventsFilterDeploys": {
    message: "部署",
    description: "Deploy events category",
  },
  "metrics.eventsFilterLifecycle": {
    message: "生命周期",
    description: "Lifecycle events category",
  },
  "metrics.eventsFilterConfig": {
    message: "配置",
    description: "Configuration events category",
  },
  "metrics.eventsLoading": {
    message: "正在加载事件…",
    description: "Event timeline loading state",
  },
  "metrics.eventsError": {
    message: "无法加载事件。",
    description: "Event timeline error state",
  },
  "metrics.eventsEmptyTitle": {
    message: "暂无事件",
    description: "Event timeline empty-state title",
  },
  "metrics.eventsEmptyBody": {
    message: "所选时间范围内没有事件。",
    description: "Event timeline empty-state body",
  },
  "metrics.eventsChanged": {
    message: "服务已更改",
    description: "Fallback event timeline label",
  },
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
    description:
      "Network metrics chart section title, with the selected quantile",
  },
  "metrics.outboundBandwidth": {
    message: "出站带宽",
    description: "Network metrics chart section title",
  },
  "metrics.monthToDateBandwidth": {
    message: "本月已使用 {amount}",
    description:
      "Footer under the Outbound Bandwidth chart, showing month-to-date egress",
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
    message: "所选时间范围内各实例的资源使用情况",
    description: "Application metrics card description",
  },
  "metrics.limitLabel": {
    message: "限额 {value}",
    description:
      "Resource limit annotation on the Memory/CPU chart headers, e.g. 'Limit 512 MiB'",
  },
  "metrics.noLimitConfigured": {
    message: "未配置资源限额——无法计算百分比",
    description:
      "Shown instead of a percentage chart when the App's pods declare no resource limit",
  },
  "metrics.targetLabel": {
    message: "目标 {value}",
    description:
      "Autoscale-target annotation on the Memory/CPU chart headers (percentage mode only), e.g. 'Target 70%'",
  },
  "metrics.diskTitle": {
    message: "磁盘",
    description: "Datastore metrics panel: disk usage chart section title",
  },
  "metrics.diskCapacityLabel": {
    message: "容量 {value}",
    description:
      "Disk usage chart header annotation showing the PVC's total capacity",
  },
  "metrics.connectionsTitle": {
    message: "活跃连接数",
    description:
      "Datastore metrics panel: Postgres active-connections chart section title",
  },
  "metrics.memoryTitle": {
    message: "内存",
    description:
      "Datastore metrics panel: Key Value used-memory chart section title",
  },
  "metrics.kvConnectionsTitle": {
    message: "连接数",
    description:
      "Datastore metrics panel: Key Value connected-clients chart section title",
  },
  "metrics.replicationLagTitle": {
    message: "复制延迟",
    description:
      "Datastore metrics panel: Postgres replication-lag chart section title",
  },
  "metrics.replicationLagPendingHA": {
    message: "不适用——无副本（启用高可用后可查看复制延迟）",
    description:
      "Shown instead of a chart for replication lag before Postgres HA is enabled (w1/m22)",
  },
  "metrics.datastoreMetricsTitle": {
    message: "指标",
    description: "Database/Key Value detail page metrics panel card title",
  },
  "metrics.datastoreMetricsDescription": {
    message: "该实例的实时资源使用情况",
    description:
      "Database/Key Value detail page metrics panel card description",
  },
  "metrics.statusCode": {
    message: "状态码",
    description: "Toolbar filter label for HTTP status code",
  },
  "metrics.statusCodeAll": {
    message: "全部",
    description: "Status-code filter option meaning no filtering",
  },
  "metrics.groupBy": {
    message: "分组",
    description: "Total Requests chart control label",
  },
  "metrics.groupByAllRequests": {
    message: "全部请求",
    description: "Group-by option: one aggregate series",
  },
  "metrics.groupByStatus": {
    message: "按状态码",
    description: "Group-by option: one series per HTTP status code",
  },
  "metrics.groupByMethod": {
    message: "按方法",
    description: "Group-by option: one series per HTTP method",
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
  "metrics.rangeLast3Hours": {
    message: "过去 3 小时",
    description: "Metrics time-range filter option",
  },
  "metrics.rangeLast6Hours": {
    message: "过去 6 小时",
    description: "Metrics time-range filter option",
  },
  "metrics.rangeLast12Hours": {
    message: "过去 12 小时",
    description: "Metrics time-range filter option",
  },
  "metrics.rangeLastDay": {
    message: "过去 1 天",
    description: "Metrics time-range filter option",
  },
};

export default zhMetrics;
