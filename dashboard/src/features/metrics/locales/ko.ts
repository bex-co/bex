import type { TranslationEntry } from "@/i18n";

const koMetrics: Record<string, TranslationEntry> = {
  "metrics.cpuCores": {
    message: "{value}코어",
    description:
      "A CPU amount in cores (a limit or the latest usage); value is a decimal such as 0.100 (w4/566)",
  },
  "metrics.eventsTitle": {
    message: "이벤트 타임라인",
    description:
      "Metrics-page timeline of service events in the selected range",
  },
  "metrics.eventsFilterLabel": {
    message: "이벤트 필터",
    description: "Accessible label for the Metrics event category filter",
  },
  "metrics.eventsFilterAll": {
    message: "모든 이벤트",
    description: "All event categories",
  },
  "metrics.eventsFilterDeploys": {
    message: "배포",
    description: "Deploy events category",
  },
  "metrics.eventsFilterLifecycle": {
    message: "수명 주기",
    description: "Lifecycle events category",
  },
  "metrics.eventsFilterConfig": {
    message: "구성",
    description: "Configuration events category",
  },
  "metrics.eventsLoading": {
    message: "이벤트를 불러오는 중…",
    description: "Event timeline loading state",
  },
  "metrics.eventsError": {
    message: "이벤트를 불러오지 못했습니다.",
    description: "Event timeline error state",
  },
  "metrics.eventsEmptyTitle": {
    message: "이벤트 없음",
    description: "Event timeline empty-state title",
  },
  "metrics.eventsEmptyBody": {
    message: "이 기간에는 아무 일도 없었습니다.",
    description: "Event timeline empty-state body",
  },
  "metrics.eventsChanged": {
    message: "서비스 변경됨",
    description: "Fallback event timeline label",
  },
  "metrics.eventMarkersLabel": {
    message: "서비스 이벤트",
    description: "Accessible label for the event-marker strip above a chart",
  },
  "metrics.eventMarkerCluster_other": {
    message: "이벤트 {count}개",
    description:
      "Accessible label for a clustered chart event marker (count badge, plural)",
  },
  "metrics.lineChartLabel_other": {
    message: "데이터 포인트 {count}개의 선 차트",
    description:
      "Accessible name of a metrics line chart; count is the number of distinct sample times (plural)",
  },
  "metrics.barChartLabel_other": {
    message: "데이터 포인트 {count}개의 막대 차트",
    description:
      "Accessible name of a metrics bar chart; count is the number of time buckets (plural)",
  },
  "metrics.barValueAt": {
    message: "{time}: {value}",
    description:
      "Accessible name of one focusable bar: its formatted value (e.g. 42 or 1.2K) and its local sample time",
  },
  "metrics.chartEmpty": {
    message: "해당 기간에 데이터 없음",
    description:
      "Default empty state of a metrics chart when the settled query returned no samples in the selected time range",
  },
  "metrics.eventMarkerHint": {
    message: "클릭하여 상세 정보 보기",
    description: "Footer line of the chart event-marker hover tooltip",
  },
  "metrics.eventDeployStarted": {
    message: "배포 시작됨",
    description: "Chart event-marker label: a deploy began",
  },
  "metrics.eventDeployLiveFor": {
    message: "{commit} 배포 라이브",
    description:
      "Chart event-marker label: a deploy went live, with its commit short id",
  },
  "metrics.eventDeployLive": {
    message: "배포 라이브",
    description: "Chart event-marker label: a deploy went live (no commit id)",
  },
  "metrics.eventDeployFailed": {
    message: "배포 실패",
    description: "Chart event-marker label: a deploy failed",
  },
  "metrics.eventDeployCanceled": {
    message: "배포 취소됨",
    description: "Chart event-marker label: a deploy was canceled",
  },
  "metrics.eventDeployEnded": {
    message: "배포 종료됨",
    description:
      "Chart event-marker label: a deploy finished with an unrecognized status",
  },
  "metrics.eventBuildStarted": {
    message: "빌드 시작됨",
    description: "Chart event-marker label: an image build began",
  },
  "metrics.eventBuildEnded": {
    message: "빌드 종료됨",
    description: "Chart event-marker label: an image build finished",
  },
  "metrics.eventPreDeployStarted": {
    message: "사전 배포 시작됨",
    description: "Chart event-marker label: the pre-deploy command began",
  },
  "metrics.eventPreDeployEnded": {
    message: "사전 배포 종료됨",
    description: "Chart event-marker label: the pre-deploy command finished",
  },
  "metrics.eventJobRunEnded": {
    message: "작업 종료됨",
    description: "Chart event-marker label: a one-off job finished",
  },
  "metrics.eventBranchDeleted": {
    message: "브랜치 삭제됨",
    description: "Chart event-marker label: the tracked Git branch was deleted",
  },
  "metrics.eventServerRestarted": {
    message: "서버 다시 시작됨",
    description: "Chart event-marker label: the service was restarted",
  },
  "metrics.eventSuspended": {
    message: "서비스 일시 중지됨",
    description: "Chart event-marker label: the service was suspended",
  },
  "metrics.eventResumed": {
    message: "서비스 재개됨",
    description: "Chart event-marker label: the service was resumed",
  },
  "metrics.eventInstanceCountChanged": {
    message: "인스턴스 수 변경됨",
    description: "Chart event-marker label: manual scaling changed the count",
  },
  "metrics.eventAutoscalingChanged": {
    message: "자동 확장 구성 변경됨",
    description: "Chart event-marker label: autoscaling settings changed",
  },
  "metrics.eventPlanChanged": {
    message: "인스턴스 유형 변경됨",
    description: "Chart event-marker label: the service's plan changed",
  },
  "metrics.networkTitle": {
    message: "네트워크 메트릭",
    description: "Network metrics card title",
  },
  "metrics.networkDescription": {
    message: "모든 인스턴스의 합계",
    description: "Network metrics card description",
  },
  "metrics.totalRequests": {
    message: "총 요청 수",
    description: "Network metrics chart section title",
  },
  "metrics.responseTimes": {
    message: "응답 시간",
    description: "Network metrics chart section title",
  },
  "metrics.percentile": {
    message: "백분위수",
    description: "Response Times section control label",
  },
  "metrics.percentileAll": {
    message: "전체",
    description:
      "Percentile dropdown option that overlays p50/p90/p99 on one chart",
  },
  "metrics.requestsCount_other": {
    message: "요청 {formatted}건",
    description:
      "Aggregate request count beside the Total Requests section title (plural; {formatted} is the locale-formatted count)",
  },
  "metrics.outboundBandwidth": {
    message: "아웃바운드 대역폭",
    description: "Network metrics chart section title",
  },
  "metrics.monthToDateBandwidth": {
    message: "이번 달 {amount} 사용",
    description:
      "Footer under the Outbound Bandwidth chart, showing month-to-date egress",
  },
  "metrics.monthToDateBandwidthError": {
    message: "이번 달 대역폭을 불러오지 못했습니다. 새로고침해 보세요",
    description:
      "Month-to-date bandwidth footer when the query failed with no value — distinct from a known zero and from the chart's bandwidthError (w4/090)",
  },
  "metrics.bandwidthDegraded": {
    message: "일부 데이터",
    description:
      "Badge on the Outbound Bandwidth chart when an egress source's health check failed inside the window (w1/m50)",
  },
  "metrics.bandwidthDegradedDetail": {
    message:
      "이 기간 중 일부 시간 동안 {sources} 이그레스 소스가 비정상이었습니다. 대역폭이 실제보다 적게 집계되었을 수 있습니다.",
    description:
      "Tooltip expanding the Partial data badge; {sources} is the comma-joined raw source tokens (http/websocket/direct)",
  },
  "metrics.bandwidthError": {
    message: "대역폭을 불러오지 못했습니다. 새로고침해 보세요",
    description:
      "Bandwidth chart body when the query itself failed (network/server error, not an empty window)",
  },
  "metrics.sourceNotConfigured": {
    message: "메트릭 소스가 구성되지 않았습니다",
    description: "Shown when bex-api reports no metrics backend is wired up",
  },
  "metrics.chartError": {
    message: "이 메트릭을 불러오지 못했습니다",
    description:
      "Shown when a metrics query fails with a real error (not a 503/no-data) — distinct from the empty 'No data in range' state (w9/m86)",
  },
  "metrics.chartThrottled": {
    message: "따라잡는 중 — 트래픽이 일시적으로 제한되었습니다",
    description:
      "Cold-load state when a metrics read was shed (RATE_LIMITED / AUTH_OVERLOADED) with no cached series — transient, not 'metrics unavailable' (w4/m100)",
  },
  "metrics.chartStaleThrottled": {
    message: "캐시된 데이터를 표시 중 — 일시적인 제한 후 새로 고치는 중입니다",
    description:
      "Warm-cache banner when a metrics refetch was throttled; cached series remain visible but must not read as current (w4/m100)",
  },
  "metrics.applicationTitle": {
    message: "애플리케이션 메트릭",
    description: "Application metrics card title",
  },
  "metrics.limitLink": {
    message: "한도",
    description:
      "Link to the Instance Type tab on the Memory/CPU chart headers, followed by the limit value when configured",
  },
  "metrics.manageScaling": {
    message: "확장 관리",
    description: "Link to the Scaling tab on the Total Instances chart header",
  },
  "metrics.noLimitConfigured": {
    message: "설정된 한도가 없어 백분율을 계산할 수 없습니다",
    description:
      "Shown instead of a percentage chart when the App's pods declare no resource limit",
  },
  "metrics.limitsVary": {
    message: "한도가 서로 다름",
    description:
      "Memory/CPU chart header annotation when the selected replicas declare different limits — each replica is percentized against its own limit, so no single limit value applies",
  },
  "metrics.percentageUnavailable": {
    message:
      "백분율을 표시할 수 없음 — 이 기간에 신뢰할 수 있는 한도가 없습니다",
    description:
      "Shown instead of a percentage chart when usage samples exist but every denominator was missing, zero, or otherwise untrustworthy (deleted pods, predated limit history, a mid-window rollout gap) — distinct from no usage samples or a source failure",
  },
  "metrics.targetLabel": {
    message: "목표 {value}",
    description:
      "Autoscale-target annotation on the Memory/CPU chart headers (percentage mode only), e.g. 'Target 70%'",
  },
  "metrics.diskTitle": {
    message: "디스크",
    description: "Datastore metrics panel: disk usage chart section title",
  },
  "metrics.diskCapacityLabel": {
    message: "용량 {value}",
    description:
      "Disk usage chart header annotation showing the datastore's billed/provisioned disk size",
  },
  "metrics.connectionsTitle": {
    message: "활성 연결",
    description:
      "Datastore metrics panel: Postgres active-connections chart section title",
  },
  "metrics.memoryTitle": {
    message: "메모리",
    description:
      "Datastore metrics panel: memory chart section title (Postgres container memory; Key Value used memory)",
  },
  "metrics.datastoreCpuTitle": {
    message: "CPU",
    description: "Datastore metrics panel: CPU usage chart section title",
  },
  "metrics.datastoreLimitLabel": {
    message: "한도 {value}",
    description:
      "Datastore CPU/memory chart header annotation showing the instance's configured limit",
  },
  "metrics.kvConnectionsTitle": {
    message: "연결",
    description:
      "Datastore metrics panel: Key Value connected-clients chart section title",
  },
  "metrics.replicationLagTitle": {
    message: "복제 지연",
    description:
      "Datastore metrics panel: Postgres replication-lag chart section title",
  },
  "metrics.replicationLagPendingHA": {
    message:
      "해당 없음 — 복제본이 없습니다 (복제 지연을 보려면 고가용성을 활성화하세요)",
    description:
      "Shown instead of a chart for replication lag before Postgres HA is enabled (w1/m22)",
  },
  "metrics.datastoreMetricsTitle": {
    message: "메트릭",
    description: "Database/Key Value detail page metrics panel card title",
  },
  "metrics.datastoreMetricsDescription": {
    message: "이 인스턴스의 실시간 리소스 사용량",
    description:
      "Database/Key Value detail page metrics panel card description",
  },
  "metrics.statusCode": {
    message: "상태 코드",
    description: "Toolbar filter label for HTTP status code",
  },
  "metrics.statusCodeAll": {
    message: "전체",
    description: "Status-code filter option meaning no filtering",
  },
  "metrics.host": {
    message: "호스트",
    description: "Network-card filter label for the request Host",
  },
  "metrics.hostAll": {
    message: "전체",
    description: "Host filter option meaning no filtering",
  },
  "metrics.path": {
    message: "경로",
    description: "Network-card filter label for the request Path",
  },
  "metrics.pathPlaceholder": {
    message: "경로…",
    description: "Placeholder for the free-text Path filter input",
  },
  "metrics.pathClear": {
    message: "경로 필터 지우기",
    description: "Accessible label for the button that clears the Path filter",
  },
  "metrics.hostPathStoreUnavailable": {
    message: "호스트 및 경로 필터와 호스트별 분석에는 로그 저장소가 필요합니다",
    description:
      "Per-section state when a host/path-filtered or host-grouped request metric hits a deployment with no durable log store (503) — served from the request-log store (Loki), unavailable here",
  },
  "metrics.groupBy": {
    message: "그룹 기준",
    description: "Total Requests chart control label",
  },
  "metrics.groupByAllRequests": {
    message: "모든 요청",
    description: "Group-by option: one aggregate series",
  },
  "metrics.groupByStatus": {
    message: "상태 코드",
    description: "Group-by option: one series per HTTP status code",
  },
  "metrics.groupByMethod": {
    message: "메서드",
    description: "Group-by option: one series per HTTP method",
  },
  "metrics.groupByHost": {
    message: "호스트",
    description: "Group-by option: one series per request host",
  },
  "metrics.memory": {
    message: "메모리",
    description: "Application metrics stat tile label",
  },
  "metrics.cpu": {
    message: "CPU",
    description: "Application metrics stat tile label",
  },
  "metrics.totalInstances": {
    message: "전체 인스턴스",
    description: "Application metrics stat tile label",
  },
  "metrics.filterPercentage": {
    message: "백분율",
    description: "Metrics filter tab: show values as a percentage",
  },
  "metrics.filterTotal": {
    message: "합계",
    description: "Metrics filter tab: show values as an absolute total",
  },
  "metrics.instanceFilter": {
    message: "인스턴스",
    description: "Application metrics instance multi-select label",
  },
  "metrics.instanceShowAll": {
    message: "모두 표시",
    description:
      "Explicit action that clears the retained instance selection back to all instances (w5/m91)",
  },
  "metrics.instanceUnavailableSuffix": {
    message: " (사용 불가)",
    description:
      "Suffix appended to a retained instance option that is no longer in the window or discovery (w5/m91)",
  },
  "metrics.instancesUnavailable_other": {
    message: "선택한 인스턴스 {count}개가 이 기간에 없습니다",
    description:
      "Note when retained instance selections have no samples in the window (plural; w5/m91)",
  },
  "metrics.noDataForSelection": {
    message: "이 기간에 선택한 인스턴스의 데이터가 없습니다",
    description:
      "Empty-result copy when an explicit instance filter yields no samples — distinct from query errors (w5/m91)",
  },
  "metrics.aggregateFilter": {
    message: "집계",
    description: "Application metrics replica aggregation control",
  },
  "metrics.aggregateRaw": {
    message: "원본",
    description: "Show one series per selected instance",
  },
  "metrics.aggregateMin": {
    message: "최소",
    description: "Minimum across selected replicas at each timestamp",
  },
  "metrics.aggregateMax": {
    message: "최대",
    description: "Maximum across selected replicas at each timestamp",
  },
  "metrics.aggregateAvg": {
    message: "평균",
    description: "Average across selected replicas at each timestamp",
  },
  "metrics.rangeLast30Minutes": {
    message: "최근 30분",
    description: "Metrics time-range filter option",
  },
  "metrics.rangeLastHour": {
    message: "최근 1시간",
    description: "Metrics time-range filter option",
  },
  "metrics.rangeLast4Hours": {
    message: "최근 4시간",
    description: "Metrics time-range filter option",
  },
  "metrics.rangeLast12Hours": {
    message: "최근 12시간",
    description: "Metrics time-range filter option",
  },
  "metrics.rangeLast24Hours": {
    message: "최근 24시간",
    description: "Metrics time-range filter option",
  },
  "metrics.rangeLast2Days": {
    message: "최근 2일",
    description: "Metrics time-range filter option",
  },
  "metrics.rangeLast7Days": {
    message: "최근 7일",
    description: "Metrics time-range filter option",
  },
  "metrics.rangeLast14Days": {
    message: "최근 14일",
    description: "Metrics time-range filter option",
  },
  "metrics.rangeLast30Days": {
    message: "최근 30일",
    description:
      "Metrics time-range filter option (w5/m56; Render plan-gates it)",
  },
  "metrics.rangeCustom": {
    message: "사용자 지정…",
    description: "Time-range dropdown option that opens the start/end picker",
  },
  "metrics.rangeCustomEdit": {
    message: "사용자 지정 범위 편집",
    description:
      "Button beside the time-range dropdown that reopens the picker for the active custom range",
  },
  "metrics.rangeCustomTitle": {
    message: "사용자 지정 범위",
    description: "Custom time-range picker dialog title",
  },
  "metrics.rangeCustomDescription": {
    message:
      "시작과 종료 시각을 직접 선택하세요. 30일을 초과하는 기간은 사용할 수 없습니다.",
    description: "Custom time-range picker dialog description",
  },
  "metrics.rangeCustomStart": {
    message: "시작",
    description: "Custom range start datetime field label",
  },
  "metrics.rangeCustomEnd": {
    message: "종료",
    description: "Custom range end datetime field label",
  },
  "metrics.rangeCustomApply": {
    message: "적용",
    description: "Custom range apply button",
  },
  "metrics.rangeCustomCancel": {
    message: "취소",
    description: "Custom range cancel button",
  },
  "metrics.rangeCustomErrorOrder": {
    message: "종료 시각은 시작 시각보다 늦어야 합니다.",
    description: "Custom range validation: end not after start",
  },
  "metrics.rangeCustomErrorTooLong": {
    message: "30일 이하의 기간을 선택하세요.",
    description: "Custom range validation: window exceeds the max query window",
  },
  "metrics.rangeCustomErrorTooEarly": {
    message: "범위는 {date} 이후에 시작해야 합니다.",
    description:
      "Custom range validation: the start is before the oldest instant the range may reach (e.g. a log plan's retention)",
  },
  "metrics.retentionWindow_other": {
    message: "{plan} 요금제에서는 메트릭을 {count}일 동안 보관합니다.",
    description:
      "Metrics range picker hint: how many days the workspace plan keeps metrics (w7/m427, from bex-api metricsRetention)",
  },
  "metrics.retentionUpgrade_other": {
    message: "{plan} 요금제는 {count}일 동안 보관합니다.",
    description:
      "Metrics range picker hint: a plan that keeps metrics longer, and its days; one sentence per longer window (w7/m427)",
  },
  "metrics.rangeLabel": {
    message: "시간 범위",
    description: "Accessible label for the time-range dropdown",
  },
  "metrics.eventsToggle": {
    message: "이벤트 타임라인 표시",
    description: "Accessible label for the Metrics event-timeline toggle",
  },
  "metrics.latencyPlanRequired": {
    message:
      "응답 지연 시간 메트릭은 Pro 요금제 이상에서 사용할 수 있습니다. 이 워크스페이스는 Hobby 요금제입니다.",
    description:
      "Response Times section on a Hobby workspace (PLAN_FEATURE_REQUIRED, w8/m138)",
  },
};

export default koMetrics;
