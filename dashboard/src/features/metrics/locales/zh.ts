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
  "metrics.eventMarkersLabel": {
    message: "服务事件",
    description: "Accessible label for the event-marker strip above a chart",
  },
  "metrics.eventMarkerCluster_other": {
    message: "{count} 个事件",
    description:
      "Accessible label for a clustered chart event marker (count badge)",
  },
  "metrics.eventMarkerHint": {
    message: "点击查看详情",
    description: "Footer line of the chart event-marker hover tooltip",
  },
  "metrics.eventDeployStarted": {
    message: "部署已开始",
    description: "Chart event-marker label: a deploy began",
  },
  "metrics.eventDeployLiveFor": {
    message: "部署已上线（{commit}）",
    description:
      "Chart event-marker label: a deploy went live, with its commit short id",
  },
  "metrics.eventDeployLive": {
    message: "部署已上线",
    description: "Chart event-marker label: a deploy went live (no commit id)",
  },
  "metrics.eventDeployFailed": {
    message: "部署失败",
    description: "Chart event-marker label: a deploy failed",
  },
  "metrics.eventDeployCanceled": {
    message: "部署已取消",
    description: "Chart event-marker label: a deploy was canceled",
  },
  "metrics.eventDeployEnded": {
    message: "部署已结束",
    description:
      "Chart event-marker label: a deploy finished with an unrecognized status",
  },
  "metrics.eventBuildStarted": {
    message: "构建已开始",
    description: "Chart event-marker label: an image build began",
  },
  "metrics.eventBuildEnded": {
    message: "构建已结束",
    description: "Chart event-marker label: an image build finished",
  },
  "metrics.eventPreDeployStarted": {
    message: "预部署已开始",
    description: "Chart event-marker label: the pre-deploy command began",
  },
  "metrics.eventPreDeployEnded": {
    message: "预部署已结束",
    description: "Chart event-marker label: the pre-deploy command finished",
  },
  "metrics.eventJobRunEnded": {
    message: "任务已结束",
    description: "Chart event-marker label: a one-off job finished",
  },
  "metrics.eventBranchDeleted": {
    message: "分支已删除",
    description: "Chart event-marker label: the tracked Git branch was deleted",
  },
  "metrics.eventServerRestarted": {
    message: "服务已重启",
    description: "Chart event-marker label: the service was restarted",
  },
  "metrics.eventSuspended": {
    message: "服务已暂停",
    description: "Chart event-marker label: the service was suspended",
  },
  "metrics.eventResumed": {
    message: "服务已恢复",
    description: "Chart event-marker label: the service was resumed",
  },
  "metrics.eventInstanceCountChanged": {
    message: "实例数量已变更",
    description: "Chart event-marker label: manual scaling changed the count",
  },
  "metrics.eventAutoscalingChanged": {
    message: "自动扩缩配置已变更",
    description: "Chart event-marker label: autoscaling settings changed",
  },
  "metrics.eventPlanChanged": {
    message: "实例类型已变更",
    description: "Chart event-marker label: the service's plan changed",
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
    message: "响应时间",
    description: "Network metrics chart section title",
  },
  "metrics.percentile": {
    message: "百分位",
    description: "Response Times section control label",
  },
  "metrics.percentileAll": {
    message: "全部",
    description:
      "Percentile dropdown option that overlays p50/p90/p99 on one chart",
  },
  "metrics.requestsCount_other": {
    message: "{formatted} 个请求",
    description:
      "Aggregate request count beside the Total Requests section title ({formatted} is the locale-formatted count)",
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
  "metrics.bandwidthDegraded": {
    message: "部分数据",
    description:
      "Badge on the Outbound Bandwidth chart when an egress source's health check failed inside the window (w1/m50)",
  },
  "metrics.bandwidthDegradedDetail": {
    message:
      "{sources} 出口数据源在此时间窗口内部分时间不健康——带宽可能被低估。",
    description:
      "Tooltip expanding the Partial data badge; {sources} is the comma-joined raw source tokens (http/websocket/direct)",
  },
  "metrics.bandwidthError": {
    message: "无法加载带宽数据——请尝试刷新",
    description:
      "Bandwidth chart body when the query itself failed (network/server error, not an empty window)",
  },
  "metrics.sourceNotConfigured": {
    message: "未配置指标数据源",
    description: "Shown when bex-api reports no metrics backend is wired up",
  },
  "metrics.chartError": {
    message: "无法加载此指标",
    description:
      "Shown when a metrics query fails with a real error (not a 503/no-data) — distinct from the empty 'No data in range' state (w9/m86)",
  },
  "metrics.applicationTitle": {
    message: "应用指标",
    description: "Application metrics card title",
  },
  "metrics.limitLink": {
    message: "限额",
    description:
      "Link to the Instance Type tab on the Memory/CPU chart headers, followed by the limit value when configured",
  },
  "metrics.manageScaling": {
    message: "管理扩缩容",
    description: "Link to the Scaling tab on the Total Instances chart header",
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
      "Disk usage chart header annotation showing the datastore's billed/provisioned disk size",
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
  "metrics.host": {
    message: "主机",
    description: "Network-card filter label for the request Host",
  },
  "metrics.hostAll": {
    message: "全部",
    description: "Host filter option meaning no filtering",
  },
  "metrics.path": {
    message: "路径",
    description: "Network-card filter label for the request Path",
  },
  "metrics.pathPlaceholder": {
    message: "路径…",
    description: "Placeholder for the free-text Path filter input",
  },
  "metrics.pathClear": {
    message: "清除路径筛选",
    description: "Accessible label for the button that clears the Path filter",
  },
  "metrics.hostPathStoreUnavailable": {
    message: "主机和路径筛选需要日志存储",
    description:
      "Per-section state when a host/path-filtered request metric hits a deployment with no durable log store (503) — served from the request-log store (Loki), unavailable here",
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
  "metrics.instanceFilter": {
    message: "实例",
    description: "Application metrics instance multi-select label",
  },
  "metrics.aggregateFilter": {
    message: "聚合",
    description: "Application metrics replica aggregation control",
  },
  "metrics.aggregateRaw": {
    message: "原始",
    description: "Show one series per selected instance",
  },
  "metrics.aggregateMin": {
    message: "最小",
    description: "Minimum across selected replicas at each timestamp",
  },
  "metrics.aggregateMax": {
    message: "最大",
    description: "Maximum across selected replicas at each timestamp",
  },
  "metrics.aggregateAvg": {
    message: "平均",
    description: "Average across selected replicas at each timestamp",
  },
  "metrics.rangeLast30Minutes": {
    message: "过去 30 分钟",
    description: "Metrics time-range filter option",
  },
  "metrics.rangeLastHour": {
    message: "过去一小时",
    description: "Metrics time-range filter option",
  },
  "metrics.rangeLast4Hours": {
    message: "过去 4 小时",
    description: "Metrics time-range filter option",
  },
  "metrics.rangeLast12Hours": {
    message: "过去 12 小时",
    description: "Metrics time-range filter option",
  },
  "metrics.rangeLast24Hours": {
    message: "过去 24 小时",
    description: "Metrics time-range filter option",
  },
  "metrics.rangeLast2Days": {
    message: "过去 2 天",
    description: "Metrics time-range filter option",
  },
  "metrics.rangeLast7Days": {
    message: "过去 7 天",
    description: "Metrics time-range filter option",
  },
  "metrics.rangeLast14Days": {
    message: "过去 14 天",
    description: "Metrics time-range filter option",
  },
  "metrics.rangeLast30Days": {
    message: "过去 30 天",
    description:
      "Metrics time-range filter option (w5/m56; Render plan-gates it)",
  },
  "metrics.rangeCustom": {
    message: "自定义…",
    description: "Time-range dropdown option that opens the start/end picker",
  },
  "metrics.rangeCustomTitle": {
    message: "自定义范围",
    description: "Custom time-range picker dialog title",
  },
  "metrics.rangeCustomDescription": {
    message: "选择绝对的开始和结束时间。不支持超过 30 天的时间窗口。",
    description: "Custom time-range picker dialog description",
  },
  "metrics.rangeCustomStart": {
    message: "开始",
    description: "Custom range start datetime field label",
  },
  "metrics.rangeCustomEnd": {
    message: "结束",
    description: "Custom range end datetime field label",
  },
  "metrics.rangeCustomApply": {
    message: "应用",
    description: "Custom range apply button",
  },
  "metrics.rangeCustomCancel": {
    message: "取消",
    description: "Custom range cancel button",
  },
  "metrics.rangeCustomErrorOrder": {
    message: "结束时间必须晚于开始时间。",
    description: "Custom range validation: end not after start",
  },
  "metrics.rangeCustomErrorTooLong": {
    message: "请选择 30 天以内的时间窗口。",
    description: "Custom range validation: window exceeds the max query window",
  },
  "metrics.rangeLabel": {
    message: "时间范围",
    description: "Accessible label for the time-range dropdown",
  },
  "metrics.eventsToggle": {
    message: "显示事件时间线",
    description: "Accessible label for the Metrics event-timeline toggle",
  },
};

export default zhMetrics;
